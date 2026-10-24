# 发布

平时推送和 PR 不运行 CI。仅推送版本标签，或手动运行 Release，才会测试、打包并上传 Mac 安装包。

## 发布新版本

```sh
node scripts/set-version.mjs 0.0.2
```

在 `CHANGELOG.md` 添加对应的 `## 0.0.2 — YYYY-MM-DD` 章节，提交后推送：

```sh
git tag -a v0.0.2 -m "Hermit 0.0.2"
git push --atomic origin main v0.0.2
```

标签必须与应用版本一致，已公开版本不覆盖。需要补发尚未公开的已有标签时，执行 `gh workflow run release.yml --ref main -f tag=v0.0.1`。

## 本地打包

安装 [构建工具](../CONTRIBUTING.md#构建-macos-应用)，在干净且已提交的工作区执行：

```sh
RELEASE_TAG=v0.0.1 node scripts/prepare-release.mjs
```

文件生成到 `bin/release/v0.0.1/`：Mac ZIP、对应源码包、发行说明与 `SHA256SUMS`。同名目录已存在时，请先移走旧产物。用 `shasum -a 256 -c SHA256SUMS` 验证附件。

发布只使用一个 macOS 任务，依次构建、运行现有测试、打包、上传并公开 Release，不使用中转构建产物或定时任务。

当前仅提供 Apple Silicon 包，最低构建目标 macOS 13，使用临时签名，尚未经过 Apple 公证。

## 随包源码与许可

自有代码保持 MIT；应用包含 Mihomo，分发须保留 GPLv3 许可和对应源码。脚本收集 Go / npm 许可、Go vendor 源码、生产 npm 包、Wails runtime TypeScript 和匹配的 React 源码。新增依赖时核查其许可，升级 React 时同步更新收集脚本中的源码版本与哈希。

源码包可按贡献指南运行 `wails3 package` 构建；开发工具与 npm 依赖仍需联网安装。修改附带 npm 依赖前先安装开发依赖，避免之后的 `npm ci` 覆盖修改。
