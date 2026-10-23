package core

import "strings"

// Prefix 是 Hermit 生成对象（listener / 手动节点代理）的名称前缀。
const Prefix = "hermit-"

// Source 标识映射的节点来源。
const (
	SourceActiveProfile = "active-profile"
	SourceManualURI     = "manual-uri"
)

// Mapping 是一条「本地端口 → 指定节点」的持久映射。
// JSON 结构与旧版（Python）state.json 完全兼容。
type Mapping struct {
	ID         string      `json:"id"`
	Port       int         `json:"port"`
	Source     string      `json:"source"`
	Target     string      `json:"target,omitempty"`
	Credential string      `json:"credential,omitempty"`
	Enabled    bool        `json:"enabled"`
	LastTest   *TestRecord `json:"lastTest,omitempty"`
}

// Listener 是注入 Mihomo 的本地监听器。
type Listener struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Listen string `json:"listen"`
	Port   int    `json:"port"`
	Proxy  string `json:"proxy"`
}

// TestRecord 是一次连接测试的摘要，不含任何凭据。
type TestRecord struct {
	Time              string   `json:"time"`
	Port              int      `json:"port"`
	ExpectedProxy     string   `json:"expectedProxy"`
	ActualChain       []string `json:"actualChain"`
	ExitIP            *string  `json:"exitIp"`
	LatencyMs         *int     `json:"latencyMs"`
	Status            string   `json:"status"`
	IdentityConfirmed bool     `json:"identityConfirmed"`
	Message           string   `json:"message"`
	MappingID         string   `json:"mappingId,omitempty"`
}

// State 是持久化状态文件 state.json 的完整内容；
// scriptHash 等私有字段仅本机使用，不返回前端。
type State struct {
	Schema           int          `json:"schema"`
	Enabled          bool         `json:"enabled"`
	Revision         int          `json:"revision"`
	AppliedRevision  *int         `json:"appliedRevision"`
	Mappings         []Mapping    `json:"mappings"`
	AppliedListeners []Listener   `json:"appliedListeners"`
	History          []TestRecord `json:"history"`
	ScriptHash       string       `json:"scriptHash,omitempty"`
	OriginalHash     string       `json:"originalHash,omitempty"`
	GeneratedHash    string       `json:"generatedHash,omitempty"`
	ClashDirectory   string       `json:"clashDirectory,omitempty"`
}

// DefaultState 返回全新状态的初始值。
func DefaultState() State {
	return State{Schema: 1, Mappings: []Mapping{}, AppliedListeners: []Listener{}, History: []TestRecord{}}
}

// Pending 报告是否存在已保存但未应用的变更。
func (s *State) Pending() bool {
	return s.Enabled && (s.AppliedRevision == nil || s.Revision != *s.AppliedRevision)
}

func isAlnum(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

// ValidateMappings 校验映射列表的基本约束，违反即整体拒绝。
func ValidateMappings(mappings []Mapping) error {
	ports, ids := map[int]bool{}, map[string]bool{}
	for _, item := range mappings {
		if item.Port < 1024 || item.Port > 65535 {
			return Fail("监听端口必须是 1024–65535 的整数")
		}
		if ports[item.Port] {
			return Fail("端口 %d 重复", item.Port)
		}
		ports[item.Port] = true
		if len(item.ID) > 32 || !isAlnum(item.ID) || ids[item.ID] {
			return Fail("映射 ID 无效或重复")
		}
		ids[item.ID] = true
		switch item.Source {
		case SourceActiveProfile:
			if item.Target == "" || strings.HasPrefix(item.Target, Prefix) {
				return Fail("请选择具体的活动代理节点")
			}
		case SourceManualURI:
		default:
			return Fail("不支持的节点来源")
		}
	}
	return nil
}
