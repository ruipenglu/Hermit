package core

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"
)

// ClashAPI 是 Service 依赖的 Clash 能力；测试用假实现替换。
type ClashAPI interface {
	Directory() string
	Generated() (map[string]any, error)
	ScriptPath() (string, error)
	ProfileID() (string, error)
	Nodes() (map[string]map[string]any, error)
	AssertBaseline(config map[string]any) error
	Validate(config map[string]any) error
	Reload(config map[string]any) error
	Selections() (map[string]string, error)
	RestoreSelections(selections map[string]string) error
	Doctor() (map[string]any, error)
	Connections() ([]any, error)
}

// PublicMapping 是对外暴露的映射（凭据只以是否存在的布尔出现）。
type PublicMapping struct {
	ID            string      `json:"id"`
	Port          int         `json:"port"`
	Source        string      `json:"source"`
	Target        string      `json:"target,omitempty"`
	Enabled       bool        `json:"enabled"`
	HasCredential bool        `json:"hasCredential"`
	LastTest      *TestRecord `json:"lastTest,omitempty"`
}

// PublicState 是对外暴露的运行状态；不含脚本哈希等本机私有字段。
type PublicState struct {
	Enabled          bool            `json:"enabled"`
	Revision         int             `json:"revision"`
	AppliedRevision  *int            `json:"appliedRevision"`
	Mappings         []PublicMapping `json:"mappings"`
	History          []TestRecord    `json:"history"`
	RecoveryRequired bool            `json:"recoveryRequired"`
	Status           string          `json:"status"`
	Errors           []string        `json:"errors"`
}

// MappingInput 是新增/编辑映射的输入；URI 留空表示沿用已保存凭据。
type MappingInput struct {
	ID      string
	Port    int
	Source  string
	Target  string
	URI     string
	Enabled bool
}

// journal 是事务日志；字段与旧版兼容，中断后据此恢复。
type journal struct {
	OldScript      string            `json:"oldScript"`
	OldRuntime     map[string]any    `json:"oldRuntime"`
	OldState       State             `json:"oldState"`
	NewHash        string            `json:"newHash"`
	ScriptPath     string            `json:"scriptPath"`
	Profile        string            `json:"profile"`
	ClashDirectory string            `json:"clashDirectory"`
	Selections     map[string]string `json:"selections"`
	NewListeners   []Listener        `json:"newListeners"`
	GeneratedHash  string            `json:"generatedHash"`
}

// Service 编排映射、配置事务与测试；所有操作经跨进程锁串行。
type Service struct {
	Clash ClashAPI
	Store *Store
	// 可替换钩子，测试注入；nil 时使用真实实现。
	PortCheck func(port int) error
	Verify    func(current, previous []Listener) error
}

// NewService 创建业务服务。
func NewService(clash ClashAPI, store *Store) *Service {
	return &Service{Clash: clash, Store: store, PortCheck: CheckPort, Verify: verifyListeners}
}

func (s *Service) portCheck(port int) error {
	if s.PortCheck == nil {
		return CheckPort(port)
	}
	return s.PortCheck(port)
}

func (s *Service) verify(current, previous []Listener) error {
	if s.Verify == nil {
		return verifyListeners(current, previous)
	}
	return s.Verify(current, previous)
}

func (s *Service) guard() error {
	if s.Store.Pending() {
		return Fail("存在未完成事务；请先执行 recover 恢复变更前状态")
	}
	return nil
}

func readScript(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil || !utf8.Valid(data) {
		return "", Fail("全局脚本不是有效 UTF-8 文本，拒绝修改")
	}
	return string(data), nil
}

// publicState 把内部状态转换为对外视图，凭据不离开核心层。
func publicState(state State) PublicState {
	public := PublicState{Enabled: state.Enabled, Revision: state.Revision,
		AppliedRevision: state.AppliedRevision, Mappings: []PublicMapping{}, History: state.History}
	for _, item := range state.Mappings {
		public.Mappings = append(public.Mappings, PublicMapping{
			ID: item.ID, Port: item.Port, Source: item.Source, Target: item.Target,
			Enabled: item.Enabled, HasCredential: item.Credential != "", LastTest: item.LastTest,
		})
	}
	return public
}

// Status 以脚本与内核实际运行情况为准推导状态。
func (s *Service) Status() (PublicState, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return PublicState{}, err
	}
	defer unlock()
	state, err := s.Store.Load()
	if err != nil {
		return PublicState{}, err
	}
	result := publicState(state)
	result.Status, result.Errors = "disabled", []string{}
	result.RecoveryRequired = s.Store.Pending()
	if result.RecoveryRequired {
		result.Status = "recovery-required"
		return result, nil
	}
	scriptPath, err := s.Clash.ScriptPath()
	if err != nil {
		result.Status = "unavailable"
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}
	script, err := readScript(scriptPath)
	if err != nil {
		result.Status = "unavailable"
		result.Errors = append(result.Errors, err.Error())
		return result, nil
	}
	if state.Enabled {
		if DigestString(script) != state.ScriptHash {
			result.Errors = append(result.Errors, "全局脚本被外部修改；已暂停写入")
		}
		nodes, err := s.Clash.Nodes()
		if err != nil {
			result.Status = "unavailable"
			result.Errors = append(result.Errors, err.Error())
			return result, nil
		}
		for _, item := range state.AppliedListeners {
			if !Listening(item.Port) {
				result.Errors = append(result.Errors, fmt.Sprintf("端口 %d 未监听", item.Port))
			}
			if !strings.HasPrefix(item.Proxy, Prefix) {
				if _, ok := nodes[item.Proxy]; !ok {
					result.Errors = append(result.Errors, fmt.Sprintf("端口 %d 的节点已失效", item.Port))
				}
			}
		}
		if len(result.Errors) > 0 {
			result.Status = "degraded"
		} else {
			result.Status = "applied"
		}
		if state.Pending() {
			result.Status = "saved-pending-apply"
		}
	} else if strings.HasPrefix(script, Marker) {
		result.Status = "conflict"
		result.Errors = append(result.Errors, "检测到未登记的 Hermit 脚本，请使用原数据目录恢复")
	}
	return result, nil
}

// SaveMapping 新增或更新映射；manual-uri 输入 URI 留空时沿用原凭据。
func (s *Service) SaveMapping(input MappingInput) (PublicState, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return PublicState{}, err
	}
	defer unlock()
	if err := s.guard(); err != nil {
		return PublicState{}, err
	}
	state, err := s.Store.Load()
	if err != nil {
		return PublicState{}, err
	}
	var old *Mapping
	for i, item := range state.Mappings {
		if item.ID == input.ID {
			old = &state.Mappings[i]
			break
		}
	}
	if input.ID != "" && old == nil {
		return PublicState{}, Fail("映射不存在")
	}
	item := Mapping{ID: input.ID, Port: input.Port, Source: input.Source, Enabled: input.Enabled}
	if item.ID == "" {
		buffer := make([]byte, 6)
		rand.Read(buffer)
		item.ID = hex.EncodeToString(buffer)
	}
	switch item.Source {
	case SourceActiveProfile:
		item.Target = input.Target
	case SourceManualURI:
		if input.URI != "" {
			proxy, label, err := ParseURI(input.URI)
			if err != nil {
				return PublicState{}, err
			}
			_ = proxy
			// 备注名仅用于界面展示，凭据仍加密保存完整 URI
			item.Target = label
			item.Credential, err = s.Store.Vault.Encrypt(strings.TrimSpace(input.URI))
			if err != nil {
				return PublicState{}, err
			}
		} else if old != nil && old.Source == SourceManualURI {
			item.Credential = old.Credential
		} else {
			return PublicState{}, Fail("请输入 VLESS URI")
		}
	}
	if err := ValidateMappings([]Mapping{item}); err != nil {
		return PublicState{}, err
	}
	// 与既有映射合并后整体校验（端口/ID 唯一）
	remaining := make([]Mapping, 0, len(state.Mappings)+1)
	for _, existing := range state.Mappings {
		if existing.ID != item.ID {
			remaining = append(remaining, existing)
		}
	}
	merged := append(remaining, item)
	if err := ValidateMappings(merged); err != nil {
		return PublicState{}, err
	}
	if item.Source == SourceActiveProfile {
		nodes, err := s.Clash.Nodes()
		if err != nil {
			return PublicState{}, err
		}
		if _, ok := nodes[item.Target]; !ok {
			return PublicState{}, Fail("目标不是当前运行中的原子代理节点")
		}
	}
	state.Mappings = merged
	state.Revision++
	if err := s.Store.Save(state); err != nil {
		return PublicState{}, err
	}
	return publicState(state), nil
}

// DeleteMapping 删除映射。
func (s *Service) DeleteMapping(ident string) (PublicState, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return PublicState{}, err
	}
	defer unlock()
	if err := s.guard(); err != nil {
		return PublicState{}, err
	}
	state, err := s.Store.Load()
	if err != nil {
		return PublicState{}, err
	}
	remaining := state.Mappings[:0:0]
	found := false
	for _, item := range state.Mappings {
		if item.ID == ident {
			found = true
			continue
		}
		remaining = append(remaining, item)
	}
	if !found {
		return PublicState{}, Fail("映射不存在")
	}
	state.Mappings = remaining
	state.Revision++
	if err := s.Store.Save(state); err != nil {
		return PublicState{}, err
	}
	return publicState(state), nil
}

// manual 解密并解析启用的手动 URI 映射。
func (s *Service) manual(state State) (map[string]map[string]any, error) {
	manual := map[string]map[string]any{}
	for _, item := range state.Mappings {
		if item.Source != SourceManualURI || !item.Enabled {
			continue
		}
		plain, err := s.Store.Vault.Decrypt(item.Credential)
		if err != nil {
			return nil, err
		}
		proxy, _, err := ParseURI(plain)
		if err != nil {
			return nil, err
		}
		manual[item.ID] = proxy
	}
	return manual, nil
}

// cleanBase 移除上一轮应用注入的对象，得到干净的生成配置基线。
func cleanBase(config map[string]any, state State) map[string]any {
	base := deepCopyValue(config).(map[string]any)
	owned := map[string]bool{}
	for _, item := range state.AppliedListeners {
		owned[item.Name] = true
	}
	for _, key := range []string{"listeners", "proxies"} {
		items := sliceOf(base[key])
		kept := items[:0:0]
		for _, raw := range items {
			if item, ok := raw.(map[string]any); ok && owned[anyString(item["name"])] {
				continue
			}
			kept = append(kept, raw)
		}
		base[key] = kept
	}
	return base
}

// original 依据当前状态取回原始脚本；存在未登记接管或外部修改时拒绝。
func (s *Service) original(state State, currentScript string) (string, error) {
	if state.Enabled {
		if state.ClashDirectory != s.Clash.Directory() {
			return "", Fail("当前 Clash 目录与启用时不一致，拒绝写入")
		}
		if DigestString(currentScript) != state.ScriptHash {
			return "", Fail("原脚本被外部修改，拒绝覆盖；请先处理冲突")
		}
		raw, err := s.Store.ReadPrivate("original.bin")
		if err != nil {
			return "", err
		}
		original, _ := raw.(string)
		if DigestString(original) != state.OriginalHash {
			return "", Fail("原始备份校验失败")
		}
		return original, nil
	}
	if strings.HasPrefix(currentScript, Marker) {
		return "", Fail("检测到没有恢复记录的 Hermit 脚本，拒绝接管")
	}
	return currentScript, nil
}

// Apply 应用或停用固定端口；enabled 为 nil 时沿用当前开关。
// 写入前建立加密事务日志，失败自动回滚，回滚失败保留恢复记录。
func (s *Service) Apply(enabled *bool) (PublicState, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return PublicState{}, err
	}
	defer unlock()
	if err := s.guard(); err != nil {
		return PublicState{}, err
	}
	state, err := s.Store.Load()
	if err != nil {
		return PublicState{}, err
	}
	target := state.Enabled
	if enabled != nil {
		target = *enabled
	}
	scriptPath, err := s.Clash.ScriptPath()
	if err != nil {
		return PublicState{}, err
	}
	currentScript, err := readScript(scriptPath)
	if err != nil {
		return PublicState{}, err
	}
	original, err := s.original(state, currentScript)
	if err != nil {
		return PublicState{}, err
	}
	if !target && !state.Enabled {
		return publicState(state), nil
	}
	generated, err := s.Clash.Generated()
	if err != nil {
		return PublicState{}, err
	}
	generatedHash := DigestString(canonicalJSON(generated))
	profile, err := s.Clash.ProfileID()
	if err != nil {
		return PublicState{}, err
	}
	if err := s.Clash.AssertBaseline(generated); err != nil {
		return PublicState{}, err
	}
	base := cleanBase(generated, state)
	var listeners []Listener
	var proxies []map[string]any
	candidate := base
	newScript := original
	if target {
		nodes, err := s.Clash.Nodes()
		if err != nil {
			return PublicState{}, err
		}
		manualProxies, err := s.manual(state)
		if err != nil {
			return PublicState{}, err
		}
		listeners, proxies, err = Additions(state.Mappings, nodes, manualProxies)
		if err != nil {
			return PublicState{}, err
		}
		oldPorts := map[int]bool{}
		if state.Enabled {
			for _, item := range state.AppliedListeners {
				oldPorts[item.Port] = true
			}
		}
		for _, listener := range listeners {
			if !oldPorts[listener.Port] {
				if err := s.portCheck(listener.Port); err != nil {
					return PublicState{}, err
				}
			}
		}
		newScript, err = ComposeScript(original, listeners, proxies)
		if err != nil {
			return PublicState{}, err
		}
		before, err := EvaluateScript(original, base, "")
		if err != nil {
			return PublicState{}, err
		}
		after, err := EvaluateScript(newScript, base, "")
		if err != nil {
			return PublicState{}, err
		}
		injected, err := Inject(before, listeners, proxies)
		if err != nil {
			return PublicState{}, err
		}
		if canonicalJSON(after) != canonicalJSON(injected) {
			return PublicState{}, Fail("无法证明脚本组合保持原逻辑，拒绝自动接管")
		}
		if candidate, err = Inject(base, listeners, proxies); err != nil {
			return PublicState{}, err
		}
	}
	if err := s.Clash.Validate(candidate); err != nil {
		return PublicState{}, err
	}
	previousRuntime := generated
	if state.Enabled {
		if raw, err := s.Store.ReadPrivate("applied.bin"); err == nil {
			previousRuntime, _ = raw.(map[string]any)
		}
		if generatedHash != state.GeneratedHash {
			previousRuntime = generated
		}
	}
	nextState := state
	nextState.Enabled = target
	nextState.AppliedRevision = nil
	if target {
		revision := state.Revision
		nextState.AppliedRevision = &revision
	}
	nextState.AppliedListeners = listeners
	nextState.ScriptHash = DigestString(newScript)
	nextState.OriginalHash = DigestString(original)
	nextState.GeneratedHash = generatedHash
	nextState.ClashDirectory = s.Clash.Directory()
	if !state.Enabled {
		if err := s.Store.WritePrivate("original.bin", original); err != nil {
			return PublicState{}, err
		}
	}
	if listeners == nil {
		listeners = []Listener{}
	}
	selections, err := s.Clash.Selections()
	if err != nil {
		return PublicState{}, err
	}
	entry := journal{OldScript: currentScript, OldRuntime: previousRuntime, OldState: state,
		NewHash: DigestString(newScript), ScriptPath: scriptPath, Profile: profile,
		ClashDirectory: s.Clash.Directory(), Selections: selections, NewListeners: listeners,
		GeneratedHash: generatedHash}
	if err := s.Store.WritePrivate("journal.bin", entry); err != nil {
		return PublicState{}, err
	}
	if err := s.commit(scriptPath, currentScript, newScript, profile, generatedHash, candidate, listeners, state, nextState); err != nil {
		if rollbackErr := s.rollback(entry); rollbackErr != nil {
			return PublicState{}, Fail("应用失败，回滚未完成；已保留恢复记录，请执行 recover。若脚本存在外部冲突，请先人工处理")
		}
		reason := "本地写入或重载异常"
		if _, ok := err.(*Error); ok {
			reason = err.Error()
		}
		return PublicState{}, Fail("%s；已恢复变更前脚本和运行配置", reason)
	}
	return publicState(nextState), nil
}

// unchanged 三方核对：脚本、订阅与生成配置在写入期间未被外部改动。
func (s *Service) unchanged(scriptPath, expected, profile, generatedHash string) error {
	script, err := readScript(scriptPath)
	if err != nil {
		return err
	}
	if DigestString(script) != expected {
		return Fail("全局脚本发生外部修改")
	}
	current, err := s.Clash.ProfileID()
	if err != nil {
		return err
	}
	generated, err := s.Clash.Generated()
	if err != nil {
		return err
	}
	if current != profile || DigestString(canonicalJSON(generated)) != generatedHash {
		return Fail("应用期间 Clash 配置发生变化")
	}
	return nil
}

func (s *Service) commit(scriptPath, currentScript, newScript, profile, generatedHash string,
	candidate map[string]any, listeners []Listener, state, nextState State) error {
	check := func(expected string) error {
		return s.unchanged(scriptPath, expected, profile, generatedHash)
	}
	if err := check(DigestString(currentScript)); err != nil {
		return err
	}
	if err := AtomicWrite(scriptPath, []byte(newScript)); err != nil {
		return err
	}
	if err := check(DigestString(newScript)); err != nil {
		return err
	}
	if err := s.Clash.Reload(candidate); err != nil {
		return err
	}
	if err := s.verify(listeners, state.AppliedListeners); err != nil {
		return err
	}
	if err := check(DigestString(newScript)); err != nil {
		return err
	}
	if err := s.Store.WritePrivate("applied.bin", candidate); err != nil {
		return err
	}
	if err := s.Store.Save(nextState); err != nil {
		return err
	}
	s.Store.ClearJournal()
	return nil
}

// verifyListeners 等待重载后监听端口与预期一致：新增端口就绪、移除端口关闭。
func verifyListeners(current, previous []Listener) error {
	wanted := map[int]bool{}
	for _, item := range current {
		wanted[item.Port] = true
	}
	removed := map[int]bool{}
	for _, item := range previous {
		if !wanted[item.Port] {
			removed[item.Port] = true
		}
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		ready := true
		for port := range wanted {
			if !Listening(port) {
				ready = false
			}
		}
		for port := range removed {
			if Listening(port) {
				ready = false
			}
		}
		if ready {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return Fail("重载后监听端口与预期不一致")
}

// rollback 按事务日志恢复变更前状态；环境已变化时拒绝覆盖。
func (s *Service) rollback(entry journal) error {
	profile, err := s.Clash.ProfileID()
	if err != nil {
		return err
	}
	if s.Clash.Directory() != entry.ClashDirectory || profile != entry.Profile {
		return Fail("Clash 环境已变更，拒绝回滚到旧订阅")
	}
	generated, genErr := s.Clash.Generated()
	if genErr != nil {
		return genErr
	}
	if DigestString(canonicalJSON(generated)) != entry.GeneratedHash {
		return Fail("Clash 生成配置已更新，拒绝用旧快照覆盖新配置")
	}
	scriptPath, pathErr := s.Clash.ScriptPath()
	if pathErr != nil {
		return pathErr
	}
	script, scriptErr := readScript(scriptPath)
	if scriptErr != nil {
		return scriptErr
	}
	if scriptPath != entry.ScriptPath ||
		(DigestString(script) != DigestString(entry.OldScript) && DigestString(script) != entry.NewHash) {
		return Fail("脚本发生外部修改，拒绝回滚覆盖")
	}
	if err := AtomicWrite(scriptPath, []byte(entry.OldScript)); err != nil {
		return err
	}
	if err := s.Clash.Reload(entry.OldRuntime); err != nil {
		return err
	}
	if err := s.Clash.RestoreSelections(entry.Selections); err != nil {
		return err
	}
	if err := s.verify(entry.OldState.AppliedListeners, entry.NewListeners); err != nil {
		return err
	}
	if err := s.Store.WritePrivate("applied.bin", entry.OldRuntime); err != nil {
		return err
	}
	if err := s.Store.Save(entry.OldState); err != nil {
		return err
	}
	s.Store.ClearJournal()
	return nil
}

// Recover 恢复未完成事务。
func (s *Service) Recover() (map[string]any, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if !s.Store.Pending() {
		return nil, Fail("没有待恢复事务")
	}
	raw, err := s.Store.ReadPrivate("journal.bin")
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, Fail("恢复数据缺失或损坏，请保留 Hermit 数据目录并检查备份")
	}
	// UseNumber 保留数字字面，恢复的运行配置才能与原配置逐字一致
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var entry journal
	if err := decoder.Decode(&entry); err != nil {
		return nil, Fail("恢复数据缺失或损坏，请保留 Hermit 数据目录并检查备份")
	}
	if err := s.rollback(entry); err != nil {
		return nil, err
	}
	return map[string]any{"status": "recovered", "message": "已恢复变更前脚本、运行配置和映射状态"}, nil
}

// Test 对指定映射执行出口与代理链核验，结果写入历史（最近 100 条）。
func (s *Service) Test(ident string) (TestRecord, error) {
	unlock, err := s.Store.Lock()
	if err != nil {
		return TestRecord{}, err
	}
	defer unlock()
	if err := s.guard(); err != nil {
		return TestRecord{}, err
	}
	state, err := s.Store.Load()
	if err != nil {
		return TestRecord{}, err
	}
	if !state.Enabled || state.Pending() {
		return TestRecord{}, Fail("请先启用并应用当前映射后再测试")
	}
	scriptPath, err := s.Clash.ScriptPath()
	if err != nil {
		return TestRecord{}, err
	}
	script, err := readScript(scriptPath)
	if err != nil {
		return TestRecord{}, err
	}
	if DigestString(script) != state.ScriptHash {
		return TestRecord{}, Fail("脚本存在外部修改，无法测试已保存映射")
	}
	var item *Mapping
	for i := range state.Mappings {
		if state.Mappings[i].ID == ident {
			item = &state.Mappings[i]
			break
		}
	}
	if item == nil || !item.Enabled {
		return TestRecord{}, Fail("映射不存在或已停用")
	}
	expected := item.Target
	if item.Source == SourceManualURI {
		expected = Prefix + item.ID
	} else {
		nodes, err := s.Clash.Nodes()
		if err != nil {
			return TestRecord{}, err
		}
		if _, ok := nodes[expected]; !ok {
			return TestRecord{}, Fail("目标节点已失效，请重新配置")
		}
	}
	record := Probe(s.Clash.Connections, item.Port, expected, TestURL, nil)
	record.MappingID = ident
	item.LastTest = &record
	state.History = append(state.History, record)
	if len(state.History) > 100 {
		state.History = state.History[len(state.History)-100:]
	}
	if err := s.Store.Save(state); err != nil {
		return record, err
	}
	return record, nil
}
