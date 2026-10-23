# Hermit

Clash Verge Rev 的固定代理端口管理器。将 `127.0.0.1:18080` 等本地端口绑定到明确节点，保留现有分流。

应用使用 **Wails v3（Go + React + TypeScript）** 构建：Go 进程内实现全部核心逻辑（配置事务、加密、回滚、代理链测试），前端经 Wails 绑定调用，无本地 HTTP 服务、无子进程、无额外运行时依赖。

## 使用应用

构建产物为 `bin/hermit.app`（`wails3 package` 生成，本地临时签名）。Clash Verge Rev 必须已安装、启动并加载有效配置。

1. 在「端口映射」中点击「新增映射」，填写端口，选择当前活动节点或输入 VLESS URI。
2. 开启顶部总开关后，新增、编辑和删除映射会自动应用；总开关关闭时仅保存。关闭总开关会恢复原全局脚本并移除 Hermit 监听，保留映射，重新启用无需重填。
3. 在浏览器或其他应用中设置 HTTP / SOCKS5 代理，如 `127.0.0.1:18080`；点击映射行的「复制 URL」可直接复制 `socks5://…` 地址。
4. 点击「测试」或「保存并测试」验证出口：显示出口 IP、耗时与实际代理链，确认是否走了指定节点。

已有旧版（Python/Swift 客户端）的映射、历史和钥匙串密钥继续使用，无需迁移。

## 架构与维护

| 位置 | 责任 |
| --- | --- |
| `internal/core/` | 全部核心：存储与 Fernet 钥匙串、Clash 适配（Unix socket 控制接口）、VLESS 解析、脚本组合与 goja 安全求值、应用事务与回滚、出口探测 |
| `internal/app/` | Wails 服务层：串行操作、保存自动应用、保存并测试等编排 |
| `frontend/src/` | React UI：侧栏、映射卡片、编辑弹窗、历史与环境页；浏览器自动切换 mock 便于界面自测 |
| `build/` + `Taskfile.yml` | Wails 构建与打包配置（`wails3 build` / `wails3 package`） |

操作串行执行，存储层保留跨进程锁，可与手动操作安全互斥。测试集中在 `internal/core/*_test.go`（61 项，含假 Clash 控制器与假内核的集成场景）。

## 开发与打包

需要 Go 1.26+（`GOTOOLCHAIN=auto` 自动下载）、Node 24、wails3 CLI 与 Apple Silicon Mac。

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
wails3 build          # 构建二进制（含前端与绑定生成）
wails3 package        # 产出 bin/hermit.app 并本地签名
wails3 dev            # 开发模式（真实后端 + 前端热更新）
go test ./...         # 核心测试
```

环境变量 `HERMIT_DATA_DIR` / `HERMIT_CLASH_DIR` 可覆盖数据目录与 Clash 目录，用于隔离测试。

### 浏览器预览 UI（快速调试样式与交互）

前端 API 层会检测运行环境：不在 Wails 窗口内（无注入的 IPC）时自动切换到 mock 实现，**不触碰真实 Clash 与本地数据**，所有页面、弹窗和操作（保存 / 测试 / 删除 / 开关 / 恢复）均可完整走查。

```sh
cd frontend
npm run dev                    # 默认 http://127.0.0.1:9245
npx vite --port 5199           # 或指定端口
```

- 改样式 / 组件后保存即热更新，无需重启；设计系统（配色、圆角、明暗主题）集中在 `frontend/src/styles.css` 的 CSS 变量里。
- 想预览特定状态（如 degraded、待应用、恢复中），直接编辑 `frontend/src/api/mock.ts` 顶部的示例映射与状态开关，保存后刷新即可。
- mock 操作带模拟延迟（保存约 0.6s、测试约 1.4s），测试结果随机产生 verified / mismatch / unconfirmed 三种状态，便于观察各样式分支。
- 需要验证真实数据链路时再切回 `wails3 dev` 或打包应用。

## 凭据与恢复

- 数据保存在 `~/Library/Application Support/Hermit/`，目录 0700、文件 0600；手动 URI 和恢复快照经 Fernet 加密，密钥保存在 macOS 钥匙串。
- 写入前组合原脚本、经安装的 Mihomo 内核校验，再建立加密事务日志；原子写入后重载并检查本地监听，保留手动策略组选择。操作失败自动回滚；中断后可在「环境与恢复」中一键恢复。
- 原脚本、活动订阅或生成配置存在外部变更时拒绝覆盖；已存在加密数据但钥匙串密钥缺失时拒绝创建替代密钥。
- 测试历史保留最近 100 条摘要，不保存原始 URI；出口测试经指定端口访问 `api.ipify.org` 并按连接详情核验节点身份。

## 兼容与验收边界

已实测 Clash Verge Rev 2.5.8 + Mihomo v1.19.32。控制接口仅使用本地 Unix socket。活动节点排除策略组和内置 DIRECT / REJECT 等；手动 URI 解析直接复用 Mihomo 内核的分享链接转换器（`common/convert`，与订阅转换同一套代码），支持 VLESS / VMess / Trojan / Shadowsocks / ShadowsocksR / Hysteria / Hysteria2 / TUIC，链接备注自动作为节点显示名。

本轮验证：核心单测与假环境集成测试、真实环境只读检查（doctor / nodes / 读取旧版真实数据）、浏览器 UI 走查、打包应用启动。真实 Clash 写入（启用/停用/测试）、Developer ID 签名与公证、Intel 构建不纳入本轮，详见 [PLAN.md](PLAN.md)。
