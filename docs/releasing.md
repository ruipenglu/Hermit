# 维护与发布

普通分支推送和 Pull Request 运行 CI。推送 `v0.0.1` 这样的版本标签，会自动测试、构建 macOS arm64 安装包，并创建同名 GitHub Release。版本由仓库中的应用版本决定，标签必须与它一致；普通提交不会自动增加版本号。

## 发布一个版本

在干净工作区执行：

```sh
node scripts/set-version.mjs 0.0.1
```

此命令同步 Go 应用、前端包及锁文件、构建配置和两个 macOS plist 的版本。开发预览从前端包读取版本。随后在 `CHANGELOG.md` 中添加 `## 0.0.1 — YYYY-MM-DD` 章节，写明变化及限制，完成检查后提交。

```sh
node scripts/check-project.mjs
node scripts/check-doc-links.mjs
git add <本次发布修改的文件>
git commit -m "Release 0.0.1"
git tag -a v0.0.1 -m "Hermit 0.0.1"
git push --atomic origin main v0.0.1
```

替换命令中的示例版本和文件列表。不要移动已经公开发布的标签；修复已发布版本应使用新版本号。

如果首次启用工作流时标签事件未触发，或需要补发尚未公开的版本，可以运行 `gh workflow run release.yml --ref main -f tag=v0.0.1`。也可在 Actions 的 Release 页面手动填写已有标签。此入口检出指定标签的源码，仍执行全部验证；不会修改原标签或覆盖已公开的 Release。

## 自动流程

[Release 工作流](../.github/workflows/release.yml) 分为两个作业：

1. **构建与验证**：在 Apple Silicon macOS runner 上核对版本、文档、Go 格式，运行前端测试、Go 竞态测试、静态检查和依赖漏洞扫描；安装与 Go 模块一致的 Wails CLI，打包并校验生成文件无漂移。生成依赖许可与对应源码，将许可附在应用内部，然后重新进行临时签名和校验。
2. **发布**：仅这个作业拥有 `contents: write` 权限。下载本次构建产物并验证 SHA256，创建草稿、上传全部附件，最后公开 Release。失败时不会发布缺少附件的正式版本；重试可继续未公开的草稿，已公开版本禁止覆盖。

所有操作必须成功才会进入发布。Actions 固定到完整提交 SHA；token 不交给应用构建步骤。同一个标签的发布任务顺序执行，不互相取消。

## 下载文件

每个版本提供：

- `Hermit-版本-macos-arm64.zip`：Mac 应用，包含许可说明；解压后将 Hermit.app 拖入 Applications。
- `Hermit-版本-source.tar.gz`：该提交的源码、构建文件、Go vendor 源码、生产 npm 包、Wails runtime TypeScript 源码及匹配的 React 上游源码。
- `dependencies.json`：Go / npm 锁定版本、提交和工具链清单。
- `RELEASE_NOTES.md`：从本版本变更记录生成的发行说明。
- `SHA256SUMS`：附件完整性校验文件。

下载附件到同一目录后，可执行 `shasum -a 256 -c SHA256SUMS` 检查完整性。

## 本地验证与源码构建

在 Apple Silicon Mac 上，安装贡献指南列出的工具后，使用已提交的干净工作区执行：

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
RELEASE_TAG=v0.0.1 node scripts/prepare-release.mjs
```

输出位于 `bin/release/v0.0.1/`；本地脚本只生成文件，不上传。同名目录存在时拒绝覆盖，重试前先检查并移走旧目录。打包只读取受版本控制的源码和锁定依赖，不包含本地运行状态或用户配置。

下载的源码包可按 [贡献指南](../CONTRIBUTING.md#构建-macos-应用) 安装工具并运行 `wails3 package`。源包附带的 Go vendor 和前端依赖源码供核查及修改；常规构建会按锁文件安装开发工具和 npm 依赖，因此仍需联网。若修改附带 npm 依赖，先安装开发依赖，再应用修改并运行前端构建，避免 `npm ci` 覆盖修改。

## 许可证与维护

Hermit 自有代码保持 [MIT](../LICENSE) 授权。完整应用包含 Mihomo，按 GPLv3 组合程序要求分发；发布时保留许可与版权声明，并在同一 Release 提供对应源码。依赖收集脚本遇到缺失许可会失败，新增或升级依赖仍需人工核查其兼容性。React 上游源码固定版本及 SHA256，升级时同步更新 `scripts/collect-release-deps.mjs`。

Wails Go 模块、CLI 和 npm runtime 需要同时更新。定期安全工作流和 Dependabot 辅助维护依赖；构建脚本不替代兼容性测试。

当前包仅针对 Apple Silicon，最低构建目标为 macOS 13，使用临时签名，尚未经过 Developer ID 签名 / Apple 公证。真实 Clash 完整写入验收和其他平台支持范围见 [PLAN.md](../PLAN.md#未验收与已知限制)，发行说明会保留这些限制。

GitHub 私密漏洞报告、Dependabot 安全更新和默认分支保护仍需维护者配置。本流程不修改这些仓库设置。
