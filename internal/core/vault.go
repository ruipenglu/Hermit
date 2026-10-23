package core

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/fernet/fernet-go"
)

// securityResult 是一次 /usr/bin/security 调用的结果。
type securityResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// keychainRunner 执行钥匙串命令；测试中可替换。
type keychainRunner func(args []string, stdin string) (securityResult, error)

func realKeychain(args []string, stdin string) (securityResult, error) {
	cmd := exec.Command("/usr/bin/security", args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	exit := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exit = exitErr.ExitCode()
		err = nil
	}
	return securityResult{stdout: out.String(), stderr: errOut.String(), exitCode: exit}, err
}

// Vault 用 Fernet 加密敏感数据，主密钥保存在 macOS 钥匙串。
// service 名与旧版一致，保证旧密钥与旧密文继续可用。
type Vault struct {
	directory string
	service   string
	cipher    *fernet.Key
	run       keychainRunner
}

// NewVault 创建指定数据目录的加密仓库。
func NewVault(directory string) *Vault {
	return &Vault{
		directory: directory,
		service:   "app.hermit.local." + DigestString(directory)[:24],
		run:       realKeychain,
	}
}

// hasEncryptedData 检查目录中是否已有加密数据，用于防止丢失钥匙串后生成替代密钥。
func (v *Vault) hasEncryptedData() bool {
	if matches, _ := filepath.Glob(filepath.Join(v.directory, "*.bin")); len(matches) > 0 {
		return true
	}
	data, err := os.ReadFile(filepath.Join(v.directory, "state.json"))
	return err == nil && strings.Contains(string(data), `"credential"`)
}

func (v *Vault) findKey() (string, int, error) {
	res, err := v.run([]string{"find-generic-password", "-a", "Hermit", "-s", v.service, "-w"}, "")
	if err != nil {
		return "", 0, err
	}
	return strings.TrimSpace(res.stdout), res.exitCode, nil
}

// key 读取（必要时创建并校验）钥匙串中的 Fernet 主密钥。
func (v *Vault) key() (*fernet.Key, error) {
	if v.cipher != nil {
		return v.cipher, nil
	}
	value, exit, err := v.findKey()
	if err != nil {
		return nil, Fail("无法读取 macOS 钥匙串；不会生成替代密钥覆盖原数据")
	}
	if exit == 44 {
		if v.hasEncryptedData() {
			return nil, Fail("已存在加密数据但钥匙串密钥缺失；拒绝创建替代密钥，请恢复原钥匙串")
		}
		key := &fernet.Key{}
		if err := key.Generate(); err != nil {
			return nil, Fail("无法生成加密密钥")
		}
		// 密钥经 stdin 传入，不进入进程参数。
		entry := "add-generic-password -a Hermit -s " + v.service + " -w " + key.Encode() + "\n"
		saved, _ := v.run([]string{"-i"}, entry)
		if saved.exitCode != 0 || strings.Contains(saved.stderr, "SecKeychain") {
			return nil, Fail("无法将加密密钥保存到 macOS 钥匙串，请检查授权")
		}
		value, exit, err = v.findKey()
		if err != nil || exit != 0 || value != key.Encode() {
			return nil, Fail("钥匙串密钥写入后校验失败")
		}
		v.cipher = key
		return key, nil
	}
	if exit != 0 {
		return nil, Fail("无法读取 macOS 钥匙串；不会生成替代密钥覆盖原数据")
	}
	key, err := fernet.DecodeKey(value)
	if err != nil {
		return nil, Fail("钥匙串中的 Hermit 密钥无效")
	}
	v.cipher = key
	return key, nil
}

// Encrypt 加密明文并返回 Fernet 令牌。
func (v *Vault) Encrypt(value string) (string, error) {
	key, err := v.key()
	if err != nil {
		return "", err
	}
	token, err := fernet.EncryptAndSign([]byte(value), key)
	if err != nil {
		return "", Fail("加密失败")
	}
	return string(token), nil
}

// Decrypt 解密 Fernet 令牌。
func (v *Vault) Decrypt(value string) (string, error) {
	key, err := v.key()
	if err != nil {
		return "", err
	}
	message := fernet.VerifyAndDecrypt([]byte(value), 0, []*fernet.Key{key})
	if message == nil {
		return "", Fail("加密数据无法解密；请检查钥匙串或恢复备份，原数据已保留")
	}
	return string(message), nil
}

// sealJSON 序列化并加密任意值。
func (v *Vault) sealJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", Fail("数据序列化失败")
	}
	return v.Encrypt(string(data))
}

// openJSON 解密并反序列化任意值。
func (v *Vault) openJSON(token string) (any, error) {
	plain, err := v.Decrypt(token)
	if err != nil {
		return nil, err
	}
	var value any
	if err := json.Unmarshal([]byte(plain), &value); err != nil {
		return nil, Fail("加密数据无法解密；请检查钥匙串或恢复备份，原数据已保留")
	}
	return value, nil
}
