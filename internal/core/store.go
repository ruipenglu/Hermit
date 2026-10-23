package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Store 管理 Hermit 数据目录：状态文件、跨进程锁与加密私有文件。
// 目录权限 0700、文件 0600，与旧版完全兼容。
type Store struct {
	Directory string
	Vault     *Vault
}

// DefaultStoreDir 是 Hermit 的默认数据目录。
func DefaultStoreDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library/Application Support/Hermit")
}

// NewStore 创建（必要时初始化）数据目录；空参数使用默认路径，vault 为 nil 时按目录派生钥匙串 service。
func NewStore(directory string, vault *Vault) (*Store, error) {
	if directory == "" {
		directory = DefaultStoreDir()
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	if err := os.Chmod(directory, 0o700); err != nil {
		return nil, fmt.Errorf("chmod: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	if vault == nil {
		vault = NewVault(resolved)
	}
	return &Store{Directory: resolved, Vault: vault}, nil
}

func (s *Store) path(name string) string { return filepath.Join(s.Directory, name) }

// Lock 获取跨进程排他锁；返回的函数用于释放。已被占用时返回业务错误。
func (s *Store) Lock() (func(), error) {
	file, err := os.OpenFile(s.path("lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, Fail("另一个 Hermit 操作正在执行，请稍后重试")
	}
	return func() {
		syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
		file.Close()
	}, nil
}

// Load 读取状态文件；不存在时返回初始状态，损坏时拒绝继续而非覆盖。
func (s *Store) Load() (State, error) {
	data, err := os.ReadFile(s.path("state.json"))
	if os.IsNotExist(err) {
		return DefaultState(), nil
	}
	if err != nil {
		return State{}, Fail("Hermit 状态文件损坏或版本不兼容；拒绝覆盖，请先恢复备份")
	}
	var state State
	if json.Unmarshal(data, &state) != nil || state.Schema != 1 || ValidateMappings(state.Mappings) != nil {
		return State{}, Fail("Hermit 状态文件损坏或版本不兼容；拒绝覆盖，请先恢复备份")
	}
	if state.Mappings == nil {
		state.Mappings = []Mapping{}
	}
	if state.AppliedListeners == nil {
		state.AppliedListeners = []Listener{}
	}
	if state.History == nil {
		state.History = []TestRecord{}
	}
	return state, nil
}

// Save 原子写入状态文件。
func (s *Store) Save(state State) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return Fail("状态序列化失败")
	}
	return AtomicWrite(s.path("state.json"), append(data, '\n'))
}

// WritePrivate 加密写入私有文件（如 original.bin / journal.bin / applied.bin）。
func (s *Store) WritePrivate(name string, value any) error {
	token, err := s.Vault.sealJSON(value)
	if err != nil {
		return err
	}
	return AtomicWrite(s.path(name), []byte(token))
}

// ReadPrivate 读取并解密私有文件。
func (s *Store) ReadPrivate(name string) (any, error) {
	token, err := os.ReadFile(s.path(name))
	if err != nil {
		return nil, Fail("恢复数据缺失或损坏，请保留 Hermit 数据目录并检查备份")
	}
	return s.Vault.openJSON(string(token))
}

// Pending 报告是否存在未完成事务日志。
func (s *Store) Pending() bool {
	_, err := os.Lstat(s.path("journal.bin"))
	return err == nil
}

// ClearJournal 删除事务日志。
func (s *Store) ClearJournal() {
	os.Remove(s.path("journal.bin"))
}
