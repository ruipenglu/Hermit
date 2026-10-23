# Hermit

Clash Verge Rev 的固定代理端口管理器。将 `127.0.0.1:18080` 等端口绑定具体节点，保留现有分流。

客户端使用 **SwiftUI 原生界面**。Swift 负责窗口、菜单、表单和应用生命周期，已验证的 Python 核心负责 Clash 配置、加密、回滚和代理链测试。两者通过子进程的标准输入输出通信，不启动浏览器、本地 HTTP 服务或 Electron 运行时。

## 使用应用

构建产物为 `dist/Hermit.app`，压缩包为 `dist/Hermit-v1.0-macos-arm64.zip`。当前为 Apple Silicon 构建，内置 Python 要求 macOS 14.3+；用户无需安装 Python 或其他运行时。复制应用到「应用程序」后双击启动。

Clash Verge Rev 必须已安装、启动并加载有效配置。

1. 在「端口映射」中点击「新增映射」（⌘N），填写端口，选择当前活动节点或输入 VLESS URI。
2. 开启顶部总开关后，新增、编辑和删除映射会自动应用。总开关关闭时仅保存，保持关闭；开启时应用全部映射。
3. 在浏览器或其他应用中设置 HTTP / SOCKS5 代理，如 `127.0.0.1:18080`。点击映射行的「复制 URL」可复制 `socks5://127.0.0.1:18080` 格式的本地代理地址。
4. 点击映射行的「测试」。界面会显示出口 IP、耗时及节点身份是否确认；展开测试记录可查看实际代理链。
5. 关闭总开关会恢复原全局脚本并移除 Hermit 监听，保留映射。重新启用无需重填。

新增和编辑弹窗也可点击「保存并测试」：保存当前字段、开启总开关并应用全部已保存映射，然后直接显示出口 IP、耗时和实际代理链。测试结束后保持启用，点击「完成」关闭弹窗；修改字段后旧结果会清除，再次测试会更新同一条映射。

「活动节点」列出真实运行节点，可直接选择「绑定端口」；「环境与恢复」提供环境诊断和数据目录入口。⌘R 刷新状态，⌘Q 或关闭主窗口退出。若有核心操作正在执行，应用会等待操作结束再退出。退出应用后已应用的端口继续由 Clash 提供服务；需要停用时先关闭总开关。

已保存的 URI 不会回填到编辑框，编辑时留空保留原节点；新输入默认使用原生安全输入框隐藏内容。已有 Hermit 的映射、历史和钥匙串密钥继续使用，无需迁移。

## 架构与维护

| 文件 | 责任 |
| --- | --- |
| `native/HermitApp.swift` | 单窗口、系统菜单、快捷键、事务执行时延迟退出 |
| `native/ContentView.swift` | 原生侧栏、映射列表、编辑表单、测试记录、环境界面 |
| `native/ClientModel.swift` | 界面状态、Codable 数据、异步子进程通信 |
| `hermit/desktop.py` | 接收一次 JSON 请求并调用已有核心操作；返回脱敏结果 |
| `hermit/service.py` | 持久映射、配置事务、回滚、恢复及测试 |
| `Hermit.spec` | 仅打包 Python 核心及依赖 |
| `scripts/build.sh` | 编译 Swift、打包核心、组装应用、验签、生成 ZIP |

每个操作启动一个内置 `hermit-core --request`，请求通过私有 stdin 传入，结果经 stdout 返回；操作结束进程退出。凭据不进入进程参数、临时文件或日志。客户端串行处理操作，存储层继续使用跨进程锁，避免与 CLI 并发写入。只有一套客户端 UI。

## 开发与打包

需要 Apple Silicon Mac、macOS 14.3+、Apple Command Line Tools（Swift 6 编译器）及 Python 3.11+。本轮使用 Swift 6.4 和 Python 3.12.2 构建。

```sh
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.lock
.venv/bin/python -m pip install -e . --no-deps
sh scripts/build.sh
open dist/Hermit.app
```

构建脚本先运行 Python 回归测试，再以 Swift 6 严格并发检查及 warnings-as-errors 编译，打包核心，执行原生 Swift 通信冒烟检查，完成本地临时签名和验签，输出完整应用与 ZIP。构建日志位于 `build/core-build.log`。

当前应用为本地临时签名，未做 Developer ID 签名和 Apple 公证。跨机器分发前需使用正式证书签名和公证，无需关闭系统安全设置。

## CLI

```sh
.venv/bin/hermit doctor
.venv/bin/hermit nodes
.venv/bin/hermit map add --port 18080 --target '实际节点名'
.venv/bin/hermit enable
.venv/bin/hermit status
.venv/bin/hermit test 映射ID
.venv/bin/hermit map update 映射ID --port 18082 --target '实际节点名'
.venv/bin/hermit apply
.venv/bin/hermit disable
.venv/bin/hermit map delete 映射ID
```

手动 URI 从标准输入读取，避免进入进程参数。终端可使用隐藏输入：

```sh
.venv/bin/python -c 'import getpass; print(getpass.getpass("VLESS URI: "))' | .venv/bin/hermit map add --port 18081 --uri-stdin
.venv/bin/hermit map update 映射ID --port 18082 --keep-uri
```

CLI 的映射增删改只保存；`apply` 在总开关开启时应用最新配置，关闭时保持关闭；`enable` 开启并应用。`--disabled` 单独停用映射。全局参数 `--data-dir`、`--clash-dir`、`--clash-app` 置于子命令前。连接测试只有完整 verified 时返回退出码 0。

## 凭据与恢复

- 数据保存在 `~/Library/Application Support/Hermit/`。目录权限 0700，文件权限 0600；手动 URI 和恢复快照经 Fernet 加密，随机主密钥保存在 macOS 钥匙串。
- 供 Clash 执行的全局脚本必须包含内核可读取的出站凭据，Hermit 将其权限设为 0600。Clash 自身生成的配置和备份也可能包含凭据，不要分享这些文件或整个数据目录。
- 测试历史保留最近 100 条摘要，不保存原始 URI。出口测试经指定 SOCKS5 端口访问 `api.ipify.org`；出口 IP 可达与节点身份确认分别判断。
- 写入前组合原脚本、执行安装内核的配置校验，再建立加密事务日志。原子写入后重载内核并检查本地监听，保留手动策略组选择。
- 操作失败时尽力恢复原脚本和运行配置。中断或回滚失败后使用界面中的恢复按钮，或执行 `.venv/bin/hermit recover`。
- 原脚本、活动订阅或生成配置存在外部变更时拒绝覆盖。保留数据目录及钥匙串，先核对 Clash 最新内容；不要删除恢复日志绕过检查。
- 已有加密数据但钥匙串密钥缺失时拒绝创建替代密钥。卸载前先停用映射并确认原脚本恢复。

## 兼容与验收边界

已实测 Clash Verge Rev **2.5.8** + Mihomo **v1.19.32**。控制接口仅使用本地 Unix socket，不连接远程控制器。活动节点排除策略组和内置 DIRECT / REJECT / PASS 等；手动 URI 支持 VLESS TCP / TLS / REALITY / WebSocket / gRPC，不支持的参数、重复字段和不匹配组合明确报错。

原全局脚本需同步返回配置对象，通过受限 QuickJS 离线检查；复杂脚本可能因运行时差异被拒绝。Clash 重新生成配置时使用持久脚本；节点缺失时停用对应 listener，保留原脚本处理和其他有效映射，不回退其他出口。

本轮验证了原生界面、Swift 与打包核心通信、两种节点来源的真实出口身份、配置恢复和事务进行中退出。具体记录见 [PLAN.md](PLAN.md)。其他传输组合有解析及内核配置校验覆盖，尚未验证所有服务器互通、Clash 真实重启 / 订阅更新事件、其他 Clash / macOS 版本和 Intel。菜单栏、开机启动等 P1 增强另行安排。
