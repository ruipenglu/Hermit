package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// builtins 是 Mihomo 内置出站，不算可选节点。
var builtins = map[string]bool{
	"direct": true, "reject": true, "rejectdrop": true, "pass": true,
	"passrule": true, "compatible": true, "dns": true,
}

// Clash 是 Clash Verge Rev / Mihomo 的适配器：只读生成配置与脚本，
// 通过本地 Unix socket 控制接口查询与重载。
// scanProcesses / serviceSocketPath 仅供测试隔离真实环境覆盖。
type Clash struct {
	DataDir           string
	AppPath           string
	scanProcesses     bool
	serviceSocketPath string
}

// DefaultClashDataDir 是 Clash Verge Rev 的默认数据目录。
func DefaultClashDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library/Application Support/io.github.clash-verge-rev.clash-verge-rev")
}

// NewClash 创建适配器；空参数使用默认路径。
func NewClash(dataDir, app string) *Clash {
	if dataDir == "" {
		dataDir = DefaultClashDataDir()
	}
	if app == "" {
		app = "/Applications/Clash Verge.app"
	}
	return &Clash{
		DataDir:           dataDir,
		AppPath:           app,
		scanProcesses:     true,
		serviceSocketPath: fmt.Sprintf("/var/run/clash-verge-service/users/%d/verge-mihomo.sock", os.Getuid()),
	}
}

// BinaryPath 返回 Mihomo 内核路径。
func (c *Clash) BinaryPath() string { return filepath.Join(c.AppPath, "Contents/MacOS/verge-mihomo") }

// ReadYAML 读取 YAML 配置为通用对象。
func ReadYAML(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, Fail("无法读取有效配置：%s", filepath.Base(path))
	}
	var value map[string]any
	if yaml.Unmarshal(data, &value) != nil || value == nil {
		return nil, Fail("无法读取有效配置：%s", filepath.Base(path))
	}
	return value, nil
}

// Generated 读取 Clash 当前生成配置。
func (c *Clash) Generated() (map[string]any, error) {
	return ReadYAML(filepath.Join(c.DataDir, "clash-verge.yaml"))
}

// ProfileID 返回当前生效的订阅 UID。
func (c *Clash) ProfileID() (string, error) {
	profiles, err := ReadYAML(filepath.Join(c.DataDir, "profiles.yaml"))
	if err != nil {
		return "", err
	}
	uid, _ := profiles["current"].(string)
	return uid, nil
}

// ScriptPath 定位 Clash 的全局扩展脚本文件；路径必须位于 profiles 目录内，
// 拒绝符号链接与越界路径，避免被诱导覆盖其他文件。
func (c *Clash) ScriptPath() (string, error) {
	profiles, err := ReadYAML(filepath.Join(c.DataDir, "profiles.yaml"))
	if err != nil {
		return "", err
	}
	items, _ := profiles["items"].([]any)
	for _, raw := range items {
		item, _ := raw.(map[string]any)
		if item == nil || item["uid"] != "Script" || item["type"] != "script" {
			continue
		}
		file, ok := item["file"].(string)
		if !ok {
			break
		}
		root := filepath.Join(c.DataDir, "profiles")
		resolvedRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			return "", Fail("全局脚本路径无效，拒绝写入")
		}
		path := filepath.Join(root, file)
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", Fail("全局脚本路径无效，拒绝写入")
		}
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || filepath.Dir(resolved) != resolvedRoot || !info.Mode().IsRegular() {
			return "", Fail("全局脚本路径无效，拒绝写入")
		}
		return path, nil
	}
	return "", Fail("没有可安全接管的 Clash 全局脚本")
}

// parseProcessList 从 ps 输出提取 verge-mihomo 的 -ext-ctl-unix 候选 socket。
func parseProcessList(output string, uid int) []string {
	var candidates []string
	mihomo := regexp.MustCompile(`^(.*/verge-mihomo(?:-alpha)?)(?:\s+(.*))?$`)
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 3 {
			continue
		}
		if _, err := strconv.Atoi(fields[0]); err != nil {
			continue
		}
		owner, err := strconv.Atoi(fields[1])
		if err != nil || (owner != 0 && owner != uid) {
			continue
		}
		match := mihomo.FindStringSubmatch(strings.Join(fields[2:], " "))
		if match == nil {
			continue
		}
		if socket := extractCtlUnix(match[2]); socket != "" {
			candidates = append(candidates, socket)
		}
	}
	return candidates
}

func extractCtlUnix(args string) string {
	fields := strings.Fields(args)
	for i := 0; i < len(fields)-1; i++ {
		if fields[i] == "-ext-ctl-unix" && !strings.HasPrefix(fields[i+1], "-") {
			return fields[i+1]
		}
	}
	return ""
}

// SocketPath 发现 Mihomo 的本地控制 Unix socket：优先运行进程参数，
// 其次服务模式路径与生成配置中的声明。
func (c *Clash) SocketPath() (string, error) {
	var candidates []string
	if c.scanProcesses {
		if output, err := exec.Command("ps", "-axo", "pid=,uid=,command=").Output(); err == nil {
			candidates = append(candidates, parseProcessList(string(output), os.Getuid())...)
		}
	}
	candidates = append(candidates, c.serviceSocketPath)
	if generated, err := c.Generated(); err == nil {
		if declared, ok := generated["external-controller-unix"].(string); ok {
			candidates = append(candidates, declared)
		}
	}
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && info.Mode()&os.ModeSocket != 0 {
			return path, nil
		}
	}
	return "", Fail("未找到运行中的 Mihomo 本地 Unix socket；请启动 Clash Verge Rev")
}

// request 调用 Mihomo 本地控制接口并解析 JSON 响应。
func (c *Clash) request(method, path string, body any) (any, error) {
	socket, err := c.SocketPath()
	if err != nil {
		return nil, err
	}
	secret := ""
	if generated, err := c.Generated(); err == nil {
		secret, _ = generated["secret"].(string)
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socket)
			},
		},
	}
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, Fail("请求构造失败")
		}
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, "http://mihomo"+path, reader)
	if err != nil {
		return nil, Fail("请求构造失败")
	}
	request.Header.Set("Content-Type", "application/json")
	if secret != "" {
		request.Header.Set("Authorization", "Bearer "+secret)
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, Fail("Mihomo 控制接口不可用；请检查 Clash 运行状态和本地控制权限")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return nil, Fail("Mihomo 控制接口不可用；请检查 Clash 运行状态和本地控制权限")
	}
	if response.StatusCode < 200 || response.StatusCode > 299 {
		return nil, Fail("Mihomo 控制接口拒绝请求（HTTP %d），请检查配置或权限", response.StatusCode)
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return nil, Fail("Mihomo 控制接口不可用；请检查 Clash 运行状态和本地控制权限")
	}
	return value, nil
}

func asMap(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

// AtomicNodes 过滤出真实原子代理节点：排除策略组、内置出站与 Hermit 生成对象。
func AtomicNodes(proxies map[string]any) map[string]map[string]any {
	result := map[string]map[string]any{}
	for name, raw := range proxies {
		proxy, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		proxyType, _ := proxy["type"].(string)
		if _, hasAll := proxy["all"]; hasAll {
			continue
		}
		if builtins[strings.ToLower(proxyType)] || strings.HasPrefix(name, Prefix) {
			continue
		}
		result[name] = proxy
	}
	return result
}

// Nodes 返回当前运行的原子节点集合。
func (c *Clash) Nodes() (map[string]map[string]any, error) {
	response, err := c.request("GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}
	proxies, _ := asMap(response)["proxies"].(map[string]any)
	return AtomicNodes(proxies), nil
}

// SortedNodeNames 返回按名称排序的节点列表，供界面展示。
func SortedNodeNames(nodes map[string]map[string]any) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// AssertBaseline 核对生成配置与内核实际运行状态一致，避免覆盖新配置。
func (c *Clash) AssertBaseline(config map[string]any) error {
	value, err := c.request("GET", "/configs", nil)
	if err != nil {
		return err
	}
	runtime := asMap(value)
	for _, key := range []string{"port", "socks-port", "mixed-port", "redir-port", "tproxy-port"} {
		if numeric(runtime[key]) != numeric(config[key]) {
			return Fail("Clash 生成配置与运行端口不一致，拒绝接管；请在 Clash 重新应用配置")
		}
	}
	mode, _ := config["mode"].(string)
	if mode == "" {
		mode = "rule"
	}
	if runtime["mode"] != mode {
		return Fail("Clash 生成配置与运行模式不一致，请先在 Clash 重新应用配置")
	}
	nodes, err := c.Nodes()
	if err != nil {
		return err
	}
	generated := map[string]any{}
	for _, raw := range sliceOf(config["proxies"]) {
		if item, ok := raw.(map[string]any); ok {
			if name := anyString(item["name"]); name != "" {
				generated[name] = item
			}
		}
	}
	for name := range AtomicNodes(generated) {
		if _, ok := nodes[name]; !ok {
			return Fail("生成配置中的节点与当前运行节点不一致，请在 Clash 重新应用配置")
		}
	}
	return nil
}

func numeric(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	default:
		return 0
	}
}

func sliceOf(value any) []any {
	items, _ := value.([]any)
	return items
}

// Validate 用已安装的 Mihomo 内核校验候选配置，通过前不落盘任何变更。
func (c *Clash) Validate(config map[string]any) error {
	binary := c.BinaryPath()
	if info, err := os.Stat(binary); err != nil || info.IsDir() {
		return Fail("找不到已安装的 Mihomo 内核")
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return Fail("Mihomo 配置校验失败；未应用变更。请检查节点参数或脚本兼容性")
	}
	temp, err := os.CreateTemp(c.DataDir, "hermit-*.yaml")
	if err != nil {
		return Fail("Mihomo 配置校验失败；未应用变更。请检查节点参数或脚本兼容性")
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return Fail("Mihomo 配置校验失败；未应用变更。请检查节点参数或脚本兼容性")
	}
	temp.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, "-t", "-d", c.DataDir, "-f", name).CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return Fail("Mihomo 配置校验超时；未应用变更")
	}
	if err != nil {
		_ = output
		return Fail("Mihomo 配置校验失败；未应用变更。请检查节点参数或脚本兼容性")
	}
	return nil
}

// Selections 读取当前所有策略组的手动选择。
func (c *Clash) Selections() (map[string]string, error) {
	response, err := c.request("GET", "/proxies", nil)
	if err != nil {
		return nil, err
	}
	proxies, _ := asMap(response)["proxies"].(map[string]any)
	selections := map[string]string{}
	for name, raw := range proxies {
		proxy, ok := raw.(map[string]any)
		if !ok || strings.ToLower(anyString(proxy["type"])) != "selector" {
			continue
		}
		if now := anyString(proxy["now"]); now != "" {
			selections[name] = now
		}
	}
	return selections, nil
}

func anyString(value any) string {
	s, _ := value.(string)
	return s
}

// RestoreSelections 恢复策略组选择；组或选项消失时明确报错。
func (c *Clash) RestoreSelections(selections map[string]string) error {
	response, err := c.request("GET", "/proxies", nil)
	if err != nil {
		return err
	}
	available, _ := asMap(response)["proxies"].(map[string]any)
	for name, target := range selections {
		group, _ := available[name].(map[string]any)
		if group == nil {
			return Fail("重载后原策略组选择不可恢复；请检查节点变化")
		}
		found := false
		for _, option := range sliceOf(group["all"]) {
			if option == target {
				found = true
				break
			}
		}
		if !found {
			return Fail("重载后原策略组选择不可恢复；请检查节点变化")
		}
		if anyString(group["now"]) != target {
			esc := (&url.URL{Path: "/proxies/" + name}).EscapedPath()
			if _, err := c.request("PUT", esc, map[string]any{"name": target}); err != nil {
				return err
			}
		}
	}
	current, err := c.Selections()
	if err != nil {
		return err
	}
	if len(current) != len(selections) {
		return Fail("策略组选择恢复后校验失败")
	}
	for name, target := range selections {
		if current[name] != target {
			return Fail("策略组选择恢复后校验失败")
		}
	}
	return nil
}

// Reload 以候选配置强制重载内核并恢复策略组选择。
func (c *Clash) Reload(config map[string]any) error {
	selections, err := c.Selections()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(config)
	if err != nil {
		return Fail("本地写入或重载异常")
	}
	if _, err := c.request("PUT", "/configs?force=true", map[string]any{"payload": string(data)}); err != nil {
		return err
	}
	return c.RestoreSelections(selections)
}

// Doctor 只读检查 Clash 环境。
func (c *Clash) Doctor() (map[string]any, error) {
	output, err := exec.Command("/usr/bin/plutil", "-extract", "CFBundleShortVersionString",
		"raw", "-o", "-", filepath.Join(c.AppPath, "Contents/Info.plist")).Output()
	if err != nil {
		return nil, Fail("没有找到 Clash Verge Rev 安装；可通过 --clash-app 指定")
	}
	config, err := c.Generated()
	if err != nil {
		return nil, err
	}
	if err := c.AssertBaseline(config); err != nil {
		return nil, err
	}
	nodes, err := c.Nodes()
	if err != nil {
		return nil, err
	}
	script, err := c.ScriptPath()
	if err != nil {
		return nil, err
	}
	profile, err := c.ProfileID()
	if err != nil {
		return nil, err
	}
	version, err := c.request("GET", "/version", nil)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"clashVersion":    strings.TrimSpace(string(output)),
		"mihomoVersion":   anyString(asMap(version)["version"]),
		"dataDirectory":   c.DataDir,
		"script":          script,
		"profile":         profile,
		"atomicNodeCount": len(nodes),
		"controller":      "local-unix-socket",
		"status":          "ready",
	}, nil
}
