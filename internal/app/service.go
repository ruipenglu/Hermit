// Package app 暴露给 Wails 前端的服务层，核心逻辑在 internal/core。
// 所有操作串行执行；总开关开启时增删改自动应用。
package app

import (
	"fmt"
	"os"
	"sync"

	core "github.com/lurp/hermit/internal/core"
)

// Version 是应用版本号。
const Version = "1.1.0"

// Service 是前端可调用的全部后端能力。
type Service struct {
	mu    sync.Mutex
	core  *core.Service
	clash *core.Clash
}

// New 创建服务实例；环境变量 HERMIT_DATA_DIR / HERMIT_CLASH_DIR 可覆盖目录，便于隔离测试。
func New() *Service {
	store, err := core.NewStore(os.Getenv("HERMIT_DATA_DIR"), nil)
	if err != nil {
		store = nil // 初始化失败在首个操作中显式报错
		_ = err
	}
	clash := core.NewClash(os.Getenv("HERMIT_CLASH_DIR"), "")
	var service *core.Service
	if store != nil {
		service = core.NewService(clash, store)
	}
	return &Service{core: service, clash: clash}
}

func (s *Service) ready() error {
	if s.core == nil {
		return fmt.Errorf("无法初始化 Hermit 数据目录，请检查权限")
	}
	return nil
}

// NodeInfo 是一个活动原子节点。
type NodeInfo struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// MappingDraft 是新增/编辑映射的前端输入；URI 留空表示沿用已保存凭据。
type MappingDraft struct {
	ID      string `json:"id"`
	Port    int    `json:"port"`
	Source  string `json:"source"`
	Target  string `json:"target"`
	URI     string `json:"uri"`
	Enabled bool   `json:"enabled"`
}

func (d MappingDraft) input() core.MappingInput {
	return core.MappingInput{ID: d.ID, Port: d.Port, Source: d.Source, Target: d.Target, URI: d.URI, Enabled: d.Enabled}
}

// SaveResult 是保存映射的结果。
type SaveResult struct {
	State   core.PublicState   `json:"state"`
	Mapping core.PublicMapping `json:"mapping"`
}

// SaveTestResult 是「保存并测试」的结果。
type SaveTestResult struct {
	State   core.PublicState   `json:"state"`
	Mapping core.PublicMapping `json:"mapping"`
	Test    *core.TestRecord   `json:"test"`
}

func lastMapping(state core.PublicState, id string) core.PublicMapping {
	for _, item := range state.Mappings {
		if item.ID == id {
			return item
		}
	}
	return core.PublicMapping{}
}

// currentStatus 读取推导后的完整状态；失败时退回已知的最近状态。
func (s *Service) currentStatus(fallback core.PublicState) core.PublicState {
	if status, err := s.core.Status(); err == nil {
		return status
	}
	return fallback
}

// Version 返回应用版本号。
func (s *Service) Version() string { return Version }

// Status 返回当前状态与映射列表。
func (s *Service) Status() (core.PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return core.PublicState{}, err
	}
	return s.core.Status()
}

// Nodes 返回当前运行的活动原子节点（按名称排序）。
func (s *Service) Nodes() ([]NodeInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	nodes, err := s.clash.Nodes()
	if err != nil {
		return nil, err
	}
	result := make([]NodeInfo, 0, len(nodes))
	for _, name := range core.SortedNodeNames(nodes) {
		proxyType, _ := nodes[name]["type"].(string)
		result = append(result, NodeInfo{Name: name, Type: proxyType})
	}
	return result, nil
}

// Doctor 只读检查 Clash 环境。
func (s *Service) Doctor() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.clash.Doctor()
}

// SaveMapping 保存映射；总开关开启时自动应用，关闭时仅保存。
func (s *Service) SaveMapping(draft MappingDraft) (SaveResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return SaveResult{}, err
	}
	state, err := s.core.SaveMapping(draft.input())
	if err != nil {
		return SaveResult{}, err
	}
	mapping := lastMapping(state, firstNonEmpty(draft.ID, lastID(state)))
	if _, err := s.core.Apply(nil); err != nil {
		return SaveResult{State: s.currentStatus(state), Mapping: mapping},
			fmt.Errorf("映射已保存，但自动应用失败：%s", err.Error())
	}
	return SaveResult{State: s.currentStatus(state), Mapping: mapping}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func lastID(state core.PublicState) string {
	if len(state.Mappings) == 0 {
		return ""
	}
	return state.Mappings[len(state.Mappings)-1].ID
}

// SaveAndTest 保存映射、开启总开关并应用，然后立即测试指定出口。
func (s *Service) SaveAndTest(draft MappingDraft) (SaveTestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return SaveTestResult{}, err
	}
	state, err := s.core.SaveMapping(draft.input())
	if err != nil {
		return SaveTestResult{}, err
	}
	id := draft.ID
	if id == "" {
		id = lastID(state)
	}
	mapping := lastMapping(state, id)
	result := SaveTestResult{Mapping: mapping}
	on := true
	if _, err := s.core.Apply(&on); err != nil {
		result.State = s.currentStatus(state)
		return result, fmt.Errorf("映射已保存，但启用失败：%s", err.Error())
	}
	record, err := s.core.Test(id)
	result.Test = &record
	result.State = s.currentStatus(state)
	if err != nil {
		return result, fmt.Errorf("已保存并应用，但测试未完成：%s", err.Error())
	}
	return result, nil
}

// DeleteMapping 删除映射；总开关开启时自动应用。
func (s *Service) DeleteMapping(id string) (core.PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return core.PublicState{}, err
	}
	state, err := s.core.DeleteMapping(id)
	if err != nil {
		return core.PublicState{}, err
	}
	if _, err := s.core.Apply(nil); err != nil {
		return s.currentStatus(state), fmt.Errorf("映射已删除，但自动应用失败：%s", err.Error())
	}
	return s.currentStatus(state), nil
}

// SetEnabled 开启或关闭总开关。
func (s *Service) SetEnabled(enabled bool) (core.PublicState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return core.PublicState{}, err
	}
	state, err := s.core.Apply(&enabled)
	if err != nil {
		return s.currentStatus(state), err
	}
	return s.currentStatus(state), nil
}

// TestMapping 对已应用的映射执行出口与代理链测试。
func (s *Service) TestMapping(id string) (core.TestRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return core.TestRecord{}, err
	}
	return s.core.Test(id)
}

// Recover 恢复未完成事务。
func (s *Service) Recover() (map[string]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.core.Recover()
}
