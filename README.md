<p align="center"><img src="assets/Hermit.png" width="112" alt="Hermit 海螺 Logo" /></p>

# Hermit

**让一个本地端口，始终连接到你指定的代理节点。**

Hermit 是面向 macOS 的 Clash Verge Rev 固定端口管理器。适合需要给不同应用、开发工具或测试环境分配独立出口的场景；原有规则分流继续由 Clash 管理。

[English](README.en.md) · [开发指南](CONTRIBUTING.md) · [故障排查](docs/troubleshooting.md) · [安全与隐私](SECURITY.md)

> **项目状态：早期开发。许可证尚未确定，当前不能视为已完成开源授权的项目。** 代码直接使用 GPLv3 的 Mihomo 组件，公开分发前必须明确项目授权和依赖义务，见 [依赖与许可证说明](THIRD_PARTY_NOTICES.md)。目前不提供已签名、公证的正式安装包。

![Hermit 界面，展示示例映射与地址复制菜单](docs/images/preview.png)

*截图使用虚构节点及文档示例 IP，不包含真实订阅或凭据。*

## 可以做什么

- **固定出口**：将 `127.0.0.1:18080` 等端口绑定到活动节点或手动分享链接。
- **按需生效**：总开关开启时自动应用修改；关闭时恢复原脚本，保留映射。
- **验证连接**：检查出口 IP、耗时及实际代理链，区分“可达”和“确实经过指定节点”。
- **方便使用**：端口搜索、HTTP / SOCKS5 地址复制、行内测试、明暗主题与窄窗口布局。
- **本地保护**：凭据与恢复快照加密，密钥保存在 macOS 钥匙串，变更支持事务回滚与中断恢复。

Hermit 需要已运行的 Clash Verge Rev，不是独立代理内核，不替代订阅管理，也不提供代理节点。

## 环境与兼容范围

| 项目 | 当前范围 |
| --- | --- |
| 平台 | macOS，Apple Silicon（arm64） |
| 最低构建目标 | macOS 13；未对每个系统版本逐一验收 |
| 已检查的组合 | Clash Verge Rev 2.5.8 / Mihomo v1.19.32；真实环境只读检查 |
| 构建工具 | Go 1.27.3、Node.js 24、npm、Xcode Command Line Tools；完整 Xcode 用于重编 Icon Composer 资源 |
| UI 框架 | Wails v3 beta.34、React、TypeScript |
| 尚未验收 | Intel、Windows / Linux 客户端、其他 Clash 版本、真实环境完整启停 / 恢复、Developer ID 签名和公证 |

核心写入流程已有假 Clash 环境的自动化测试；这不等同于完成所有真实环境兼容测试。具体记录在 [PLAN.md](PLAN.md)。

## 从源码运行

在安装好上述工具的 Mac 上执行：

```sh
git clone https://github.com/ruipenglu/Hermit.git
cd Hermit
# 使用 nvm 时：nvm install && nvm use
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 package
open bin/hermit.app
```

打包流程按锁文件安装前端依赖、生成绑定、构建 `.app`，并进行本地临时签名。临时签名不等于 Apple 公证；不要为运行它而关闭系统整体安全保护。安装方法与构建排错见 [开发指南](CONTRIBUTING.md)。

## 使用流程

1. 启动 Clash Verge Rev，加载有效配置。
2. 点击「新增映射」，填写本地端口（1024–65535），选择活动节点或粘贴分享链接。
3. 开启「端口映射总开关」。总开关关闭时，「保存」只保存配置；**「保存并测试」会开启总开关并应用所有已启用的映射**。
4. 在目标应用中配置 `127.0.0.1` 和对应端口。点击端口旁的复制图标，可复制 `socks5://…` 或 `http://…` 地址。
5. 点击「测试」检查实际出口。仅出口可达不代表指定节点已验证，请同时查看节点验证结果。

支持 VLESS、VMess、Trojan、Shadowsocks、ShadowsocksR、Hysteria、Hysteria2、TUIC 分享链接。具体解析遵循内置的 Mihomo 转换器；不是把任意 HTTP URL 当成节点导入。

## 数据与网络行为

- 应用数据位于 `~/Library/Application Support/Hermit/`；目录权限 0700，文件 0600。
- 手动节点 URI、脚本和配置恢复快照经 Fernet 加密。钥匙串密钥缺失时拒绝生成替代密钥覆盖原数据。
- 控制 Clash 使用本地 Unix socket；启用映射会修改 Clash 的全局脚本并触发重载。
- 连接测试通过指定代理访问 `https://api.ipify.org`。该服务会看到代理出口 IP。
- 应用没有内置遥测或自动更新。节点名、端口和测试摘要仍可能是敏感信息，反馈问题前应脱敏。
- 核心逻辑在 Go 进程内执行；系统集成会调用 `security`、`ps`、`plutil` 和 Mihomo 配置校验命令，不需要额外 Python 运行时。

升级前的备份、停止使用及恢复方法见 [故障排查](docs/troubleshooting.md)。不要在映射仍启用时直接删除数据目录。

## 开发与参与

```sh
# 浏览器 UI 预览：只有开发模式使用示例数据，不访问真实 Clash
cd frontend
npm ci
npm run dev

# 前端检查
npm run typecheck
npm test
npm run build

# 在仓库根目录运行核心检查（测试使用临时目录和假环境）
cd ..
go test -race ./internal/...
go vet ./...
```

浏览器预览默认在 `http://127.0.0.1:9245/`。生产构建始终使用真实后端，不能用 `vite preview` 代替 mock 预览。`wails3 dev` 会运行真实后端，行为与纯浏览器预览不同。

提交问题前请搜索 [已有 Issues](https://github.com/ruipenglu/Hermit/issues)，按模板提供版本、复现步骤和脱敏信息。涉及凭据泄露或配置越权时，请先阅读 [安全报告流程](SECURITY.md)。

- [贡献说明](CONTRIBUTING.md)：环境、检查命令、PR 范围与测试隔离。
- [架构说明](docs/architecture.md)：数据流、事务边界与模块职责。
- [维护与发布](docs/releasing.md)：版本、构建、校验和及尚未完成的发布条件。
- [变更记录](CHANGELOG.md)：尚未正式发布的改进。
- [开发计划](PLAN.md)：当前方案、阶段验收及已知限制的唯一记录。
