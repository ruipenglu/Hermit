# 维护与发布

本项目当前没有完成授权和正式分发准备。脚本只准备本地候选文件，不创建 GitHub Release、不上传文件、不推送标签。阶段状态以 [PLAN.md](../PLAN.md) 为准。

## 首次发布前

- 由维护者确认项目许可证，添加根 `LICENSE`。直接使用 Mihomo GPLv3 组件的问题见 [第三方说明](../THIRD_PARTY_NOTICES.md)。
- 汇集实际分发依赖的版权声明和许可全文到根 `licenses/`，核查间接依赖。依据最终授权方案准备完整对应源码、依赖源码、构建脚本及再分发说明；仅附 Hermit 的 Git 源码压缩包不能代替这项审查。
- 在专用环境完成真实 Clash 的启用、停用、节点切换、更新、异常回滚与重启恢复验收。不要拿日常订阅做破坏性测试。
- 决定 Developer ID 签名和 Apple 公证方案。当前构建只有临时签名，不能宣传为 Apple 已验证的安装包。
- 明确已验收的 macOS / Clash 版本，以及未支持的架构。提交版本号不代表完成兼容测试。

## 仓库设置

仓库内工作流无法代替 GitHub 设置。维护者需在远端核对：

- 启用私密漏洞报告，再更新 [安全文档](../SECURITY.md) 的渠道说明。
- 启用 Dependabot 安全更新；当前已有定期版本更新配置。
- 给默认分支配置保护规则，要求 PR 和 `Frontend checks`、`macOS build and core tests` 成功后合并。
- 保留现有 secret scanning / push protection；使用最小权限的 Actions token。
- 填写仓库简介和主题，正式发布后再添加准确的下载入口和状态徽章。

这些设置未由本轮本地修改自动开启。不要在 CI 还未实际运行时写“CI 已通过”。

## 日常版本维护

1. 在同一改动中更新 `internal/app/service.go` 的 `Version`、前端 `package.json` / 锁文件、`build/config.yml` 和两个 macOS plist 的版本。
2. 执行 `node scripts/check-project.mjs` 检查一致性；将 `CHANGELOG.md` 的已验收条目整理到对应版本下。未发布内容保留在 Unreleased。
3. 按 [贡献说明](../CONTRIBUTING.md) 完成本地检查和 CI，记录 `PLAN.md` 中的验收结果并提交。
4. Wails Go 模块、CLI 和 npm runtime 作为一组更新。Dependabot 不单独升级 Wails；人工更新时重新生成绑定并运行完整检查。
5. Go 使用 `go.mod` 指定的安全补丁基线；发现新的标准库安全公告后及时更新并重新打包。工作流每周执行依赖审查，也可手动触发。

GitHub Actions 固定到完整提交 SHA，版本注释便于阅读；Dependabot 可提出 Actions 更新。CI 只上传测试覆盖率，不上传尚未确认许可的应用包。

## 生成本地候选包

许可证和依赖许可材料齐备、工作区已提交后，在 Apple Silicon Mac 上执行：

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
node scripts/prepare-release.mjs
```

脚本在缺少 `LICENSE` 或 `licenses/` 时立即拒绝执行；它不会替维护者判断许可是否充分。随后执行元数据、文档、前端测试、Go 竞态测试、静态检查、依赖审查、打包与签名校验。所有步骤成功后在 `bin/release/Hermit-版本-提交/` 生成：

- macOS arm64 应用 ZIP，包含 README、许可材料和文档。
- 当前提交的源码压缩包；不包含未跟踪文件、用户配置和本地状态。
- `dependencies.json`：Go / npm 锁定版本与工具链清单，不宣称为完整 SPDX SBOM。
- `SHA256SUMS`：用于下载完整性校验。

```sh
cd bin/release/Hermit-版本-提交
shasum -a 256 -c SHA256SUMS
```

同名输出目录已存在时脚本拒绝覆盖。需要重试时先检查旧文件并移动该目录。签名、公证或重新压缩都会改变哈希，最终发布文件确定后必须重新生成并验证校验和。

正式发布前还要审查候选内容和依赖对应源码、核对版本标签与提交、在干净 Mac 上验收安装和启动。确认后由维护者建立版本标签和 Release，附上变更说明、支持范围、许可和安装说明。该步骤没有自动化执行，当前也未发布候选包。
