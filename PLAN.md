# Hermit macOS 客户端开发计划

产品范围依据 `Hermit_v1.0.pdf`。每个阶段验证完成后创建一次本地提交。

## 当前方案（Wails v3 重写）

- **Wails v3（beta.34）+ Go 1.26 + React + TypeScript + Vite** 单应用重写：Go 进程内实现全部核心逻辑，前端经 Wails 绑定调用，不再通过子进程 stdin/stdout，不再依赖 Python / Swift / PyInstaller。
- 需求来自现有实现与产品 PDF，重新设计实现，不做逐文件翻译：核心域为「映射 → 脚本合成 → 校验 → 重载 → 探测」闭环，全部保留（总开关、自动应用、事务回滚、恢复、加密、冲突保护、出口核验）。
- **数据完全兼容**：沿用 `~/Library/Application Support/Hermit/` 的 `state.json`、加密 `original/journal/applied.bin` 与钥匙串密钥（Fernet + `security` CLI，同一 service 名），旧数据无需迁移。
- 脚本安全求值由 QuickJS 换为 **goja**（纯 Go、无 cgo），保留 1 秒中断时限；无内存硬限制为已接受的差异。
- UI 全新现代设计（非照搬 SwiftUI）：侧栏导航 + 状态卡 + 卡片列表 + 弹窗编辑，支持明暗外观，手写 CSS 不引 UI 框架。
- 操作在 Go 侧串行执行（互斥），存储层继续跨进程 flock，退出等待在途操作完成。

## 需求要点（重写必须覆盖）

1. 总开关：启用 = 备份原脚本 → 合成 → 校验 → 重载 → 监听核验；关闭 = 恢复原脚本并移除监听，保留映射；重新启用幂等。
2. 映射 CRUD：端口 1024–65535 整数唯一且未占用；来源为活动原子节点（排除策略组/内置）或手动 VLESS URI（严格参数校验，未知参数报错不忽略）；启用中增删改自动应用，关闭时仅保存。
3. 应用管线：运行基线核对（端口/模式/节点）、名称与端口冲突检查、`mihomo -t` 校验、原子写脚本、Unix socket 重载、恢复策略组选择、监听核验；组合脚本必须与「注入原结果」等价，否则拒绝接管。
4. 事务与恢复：加密日志、外部变更（脚本/生成配置/profile/目录）拒绝覆盖、回滚与 recover、损坏状态拒写。
5. 连接测试：SOCKS5 握手 → HTTPS 访问 `api.ipify.org` → 按源端口匹配连接链 → verified / mismatch / unconfirmed；历史保留 100 条且不含凭据。
6. 安全：目录 0700、文件 0600、URI 与快照 Fernet 加密、密钥在钥匙串、URI 不回显、错误信息不回显凭据。
7. 状态以脚本与内核实际运行情况为准：disabled / applied / saved-pending-apply / degraded / unavailable / conflict / recovery-required。
8. UI：总开关、映射列表（状态、复制 URL、测试）、新增/编辑（双来源、安全输入、留空保留 URI）、保存并测试、活动节点、测试历史、环境诊断与恢复、忙碌与错误提示、自动刷新。

## 阶段与验收

| 阶段 | 内容与验收 | 状态 |
| --- | --- | --- |
| 1 | Wails v3 + React/TS 骨架可构建出窗口，绑定链路打通 | 完成 |
| 2 | Go 存储层：state/Fernet 钥匙串/flock/原子写/日志，单测通过 | 完成 |
| 3 | Clash 集成层：yaml/脚本发现/socket 发现/节点/校验/重载/doctor，假环境单测通过 | 完成 |
| 4 | VLESS 解析器：参数与错误用例对齐旧测试，单测通过 | 完成 |
| 5 | 脚本合成与 goja 求值：组合等价、越权拒绝、超时中断，单测通过 | 完成 |
| 6 | Service 业务层 + 假环境集成测试：启停/CRUD/回滚/recover/测试探测/加密 | 待验收 |
| 7 | Wails 服务绑定 + 前端 API（串行操作、事件、浏览器 mock 自测） | 待验收 |
| 8 | 现代 UI 全部页面 + 浏览器自测（mock 数据走查四页与弹窗） | 待验收 |
| 9 | 打包 .app、真实环境只读检查、清理 Python/Swift 旧实现、README 更新 | 待验收 |

## 验证边界

- 写入路径（apply/回滚/recover/探测）全部通过**假 Clash 环境**（临时目录 + 假 yaml/脚本/内核 + Unix socket 假控制器）验证，不动真实 Clash 配置。
- 真实环境仅做**只读**检查（doctor/nodes/status）。
- 真实 Clash 的写入验收、Developer ID 签名 / 公证、Intel、其他 Clash 版本不纳入本轮，明确记录为未验收。

## 验收结果

- 阶段 2（存储层）：`go test ./internal/core/` 通过（状态/锁/加密/校验/原子写/符号链接拒绝）。跨实现互通已验证：Go fernet-go ↔ Python cryptography 双向解密对方密文；旧 Python 写入的 state.json 可被 Go 读取，Go 写入的（含 lastTest 可空字段）可被旧 Python 读取。
- 阶段 3（Clash 集成层）：假控制器（Unix socket HTTP 服务）+ 假数据目录 + 假内核测试通过：生成配置/脚本路径（含符号链接与越界拒绝）/socket 发现（进程参数解析 + 生成配置回退）/节点过滤（排除策略组、内置、hermit- 前缀，保留中文节点）/基线核对/内核校验/重载恢复策略组选择/doctor。测试禁用真实进程扫描与服务模式路径，避免触碰本机真实运行的 Clash。
- 阶段 4（VLESS 解析器）：旧测试全部用例移植通过（TLS/REALITY/WS/gRPC、未知参数拒绝、凭据不回显、IPv6）。与旧 Python 实现输出逐字段对照一致。
- 阶段 5（脚本合成与求值）：组合包装保留原作用域/参数/规则，订阅节点缺失时移除对应监听器且不影响其他映射，原脚本失败/抛错/死循环（1 秒中断）安全拒绝且不回显内容；注入保持无关字段并在名称/端口冲突时整体拒绝。真实环境只读验证：用户真实全局脚本（箭头函数 + const）+ 真实生成配置在 goja 下求值成功。
