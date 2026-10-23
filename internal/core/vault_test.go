package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/fernet/fernet-go"
)

// fakeKeychain 模拟 /usr/bin/security 的 find/add 行为。
type fakeKeychain struct {
	mu       sync.Mutex
	entries  map[string]string
	findHits int
	addCalls int
	failAdd  bool
}

func (f *fakeKeychain) run(args []string, stdin string) (securityResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(args) > 0 && args[0] == "find-generic-password" {
		f.findHits++
		service := args[4]
		if value, ok := f.entries[service]; ok {
			return securityResult{stdout: value + "\n", exitCode: 0}, nil
		}
		return securityResult{exitCode: 44}, nil
	}
	if len(args) == 1 && args[0] == "-i" {
		f.addCalls++
		if f.failAdd {
			return securityResult{exitCode: 51, stderr: "SecKeychain ... error"}, nil
		}
		fields := strings.Fields(strings.TrimSpace(stdin))
		// add-generic-password -a Hermit -s <service> -w <key>
		f.entries[fields[4]] = fields[6]
		return securityResult{exitCode: 0}, nil
	}
	return securityResult{exitCode: 1}, nil
}

func TestVaultGeneratesKeyAndReusesIt(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeKeychain{entries: map[string]string{}}
	vault := NewVault(dir)
	vault.run = fake.run

	token, err := vault.Encrypt("secret-value")
	if err != nil {
		t.Fatal(err)
	}
	if fake.addCalls != 1 || fake.findHits != 2 {
		t.Fatalf("钥匙串调用异常: add=%d find=%d", fake.addCalls, fake.findHits)
	}
	service := "app.hermit.local." + DigestString(dir)[:24]
	if _, ok := fake.entries[service]; !ok {
		t.Fatalf("密钥未写入预期 service: %s", service)
	}
	plain, err := vault.Decrypt(token)
	if err != nil || plain != "secret-value" {
		t.Fatalf("解密失败: %v %q", err, plain)
	}
	// 第二次加解密不应再触碰钥匙串（cipher 已缓存）
	before := fake.findHits
	if _, err := vault.Encrypt("again"); err != nil {
		t.Fatal(err)
	}
	if fake.findHits != before {
		t.Fatal("缓存后仍访问钥匙串")
	}
}

func TestVaultLostKeyRefusesReplacementForExistingData(t *testing.T) {
	dir := t.TempDir()
	// 目录中已有加密数据与含凭据的状态文件
	os.WriteFile(filepath.Join(dir, "original.bin"), []byte("token"), 0o600)
	fake := &fakeKeychain{entries: map[string]string{}}
	vault := NewVault(dir)
	vault.run = fake.run

	_, err := vault.Encrypt("x")
	if err == nil || !strings.Contains(err.Error(), "密钥缺失") {
		t.Fatalf("期望密钥缺失错误, got %v", err)
	}
	if fake.findHits != 1 || fake.addCalls != 0 {
		t.Fatalf("不应生成替代密钥: find=%d add=%d", fake.findHits, fake.addCalls)
	}
}

func TestVaultDetectsCredentialInStateFile(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"mappings":[{"credential":"ct"}]}`), 0o600)
	vault := NewVault(dir)
	if !vault.hasEncryptedData() {
		t.Fatal("state.json 含 credential 应视为已有加密数据")
	}
}

func TestVaultRejectsInvalidStoredKeyAndToken(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeKeychain{entries: map[string]string{}}
	vault := NewVault(dir)
	vault.run = fake.run

	fake.entries[vault.service] = "not-a-fernet-key"
	if _, err := vault.Encrypt("x"); err == nil || !strings.Contains(err.Error(), "密钥无效") {
		t.Fatalf("期望密钥无效错误, got %v", err)
	}

	fake.entries[vault.service] = testKey
	if _, err := vault.Decrypt("not-a-token"); err == nil || !strings.Contains(err.Error(), "无法解密") {
		t.Fatalf("期望解密失败错误, got %v", err)
	}
}

func TestVaultServiceNameMatchesLegacyDerivation(t *testing.T) {
	vault := NewVault("/opt/hermit-data")
	// service = "app.hermit.local." + sha256(目录字符串)[:24]
	want := "app.hermit.local." + DigestString("/opt/hermit-data")[:24]
	if vault.service != want {
		t.Fatalf("service = %s, 期望 %s", vault.service, want)
	}
}

// 确认固定密钥可用于跨实现互操作（Go 侧基准）。
func TestFernetFixedKeyRoundtrip(t *testing.T) {
	key, _ := fernet.DecodeKey(testKey)
	token, err := fernet.EncryptAndSign([]byte("interop"), key)
	if err != nil {
		t.Fatal(err)
	}
	if got := fernet.VerifyAndDecrypt(token, 0, []*fernet.Key{key}); string(got) != "interop" {
		t.Fatal("固定密钥往返失败")
	}
}
