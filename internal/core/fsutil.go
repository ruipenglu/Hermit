package core

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Digest 计算 SHA-256 十六进制摘要。
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// DigestString 计算字符串摘要。
func DigestString(s string) string { return Digest([]byte(s)) }

// AtomicWrite 经临时文件原子替换写入：目标为符号链接时拒绝覆盖，
// 写入后先同步文件再替换并同步目录，保证崩溃后不出现半写状态。
func AtomicWrite(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return Fail("拒绝覆盖符号链接")
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".hermit-*")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	name := temp.Name()
	defer os.Remove(name)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("open dir: %w", err)
	}
	defer dir.Close()
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync dir: %w", err)
	}
	return nil
}
