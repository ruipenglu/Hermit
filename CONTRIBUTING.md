# 参与 Hermit

欢迎报告可复现的问题、改善文档或提交小而完整的修复。Hermit 自有代码采用 [MIT 许可证](LICENSE)，提交的原创贡献沿用该许可证。引入第三方代码时保留其版权及许可，并更新 [第三方依赖说明](THIRD_PARTY_NOTICES.md)。

## 开发环境

- macOS / Apple Silicon，Go 1.27.3，Node 24（根目录 `.nvmrc`），npm。
- Xcode Command Line Tools：使用 Apple 官方方式安装；完整 Xcode 用于 Icon Composer。
- Wails CLI、Go 模块与 `@wailsio/runtime` 必须同步到同一版本，当前为 `v3.0.0-beta.34`。

## 构建 macOS 应用

安装上述工具后，在终端执行：

```sh
git clone https://github.com/ruipenglu/Hermit.git
cd Hermit
# 使用 nvm 时：nvm install && nvm use
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 package
open bin/hermit.app
```

打包流程按锁文件安装依赖、生成绑定并构建 `.app`。当前使用本地临时签名，尚未经过 Developer ID 签名或 Apple 公证。构建排错见 [故障排查](docs/troubleshooting.md)。

## 浏览器开发预览

在仓库根目录执行：

```sh
cd frontend
npm ci
npm run dev
```

预览地址默认为 `http://127.0.0.1:9245/`。只有开发模式的浏览器预览使用 mock，不能用 `vite preview` 代替；`wails3 dev` 会运行真实后端。

浏览器开发预览不会写入真实配置。前端单测和类型检查可在 Linux 上运行；完整客户端构建以 macOS 为准。不要修改生成绑定来修复业务问题：修改 Go 服务后运行 `wails3 generate bindings -clean=true -ts -i`，提交生成结果。

## 提交前检查

在根目录执行：

```sh
node scripts/check-project.mjs
node scripts/check-doc-links.mjs
npm run typecheck --prefix frontend
npm test --prefix frontend
npm run build --prefix frontend
node scripts/check-production.mjs
go test -race ./internal/...
go vet ./...
wails3 package
git diff --check
```

Go 使用 `gofmt`。测试新行为和重要边界，不用重复实现细节来凑数量。涉及原生 UI、数据兼容、配置写入或恢复的修改，要明确记录哪些场景做过验证、哪些没有。

CI 在 PR 上执行前端检查、核心竞态测试和 macOS 打包。依赖安全检查单独运行，结果随漏洞数据库更新，不等于项目绝对安全。

## 测试隔离

- 使用 `t.TempDir()`、测试 Vault 和假 Clash 控制器；不要读取测试者的订阅、节点、钥匙串或运行配置。
- `HERMIT_DATA_DIR` / `HERMIT_CLASH_DIR` 可覆盖目录，但**仅设置它们不等于完全隔离真实 Clash**：控制器发现仍可能检查进程或系统服务。自动化测试必须使用现有假环境并关闭真实发现路径。
- 修改生产配置、启停代理、调用真实连接测试应由操作者明确执行，不能藏在测试或构建脚本里。
- UI 截图只用 mock。示例地址使用 `example.com` 和文档保留 IP 段；不放真实节点 URI、订阅或访问令牌。

## Pull request

1. 先描述问题和可观察的预期行为。较大设计调整先开 Issue 讨论。
2. 一次 PR 解决一个明确问题；避免顺便格式化无关文件或引入未来才需要的抽象。
3. 在 `PLAN.md` 更新当前方案与验收结果；不要维护第二套开发计划。
4. PR 说明包含变更效果、验证命令、兼容性影响和剩余限制。UI 修改附脱敏截图。
5. 保持数据格式、Bundle ID 和钥匙串标识兼容；如必须变更，提供迁移与回滚设计。

## 依赖与版本

使用 `node scripts/set-version.mjs 0.0.1` 同步应用各处版本，并在 CHANGELOG 中添加对应章节。推送同版本的 `v*` 标签触发自动发布，具体流程见 [维护与发布](docs/releasing.md)。

npm 使用锁文件和 `npm ci`；依赖升级提交 `package.json` 与 `package-lock.json`。不要把 Wails runtime 改回 `latest`。Dependabot 对 Wails 的自动升级已排除，需同时更新 Go、JS 和 CLI 并验证原生窗口。

仓库采取尊重、具体的讨论方式。请针对行为和代码提出反馈，不发布个人信息，不进行骚扰或人身攻击。漏洞报告走 [SECURITY.md](SECURITY.md)，不要先公开利用细节。

参与讨论请遵守 [社区行为准则](CODE_OF_CONDUCT.md)。
