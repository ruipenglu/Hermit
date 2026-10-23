# Hermit

macOS 上的 Clash Verge Rev 固定代理端口管理器。将 `127.0.0.1:18080` 等独立端口绑定具体节点，保留现有分流。

开发顺序和验收状态见 [PLAN.md](PLAN.md)。客户端使用 SwiftUI 原生界面，复用 Python 核心，通过应用内子进程的私有管道通信。

## 使用应用

本次构建产物是 `dist/Hermit.app`（Apple Silicon，当前内置 Python 要求 macOS 14.3+），自带 Python 及依赖，可复制到「应用程序」后双击启动。它会打开独立的原生 macOS 窗口，支持 Dock、菜单和快捷键；不启动浏览器或 HTTP 服务，不需要另外安装 Python。Clash Verge Rev 必须已安装、启动并加载有效配置。

1. 在「端口映射」中添加端口，选择当前活动节点，或粘贴 VLESS URI。
2. 保存映射后开启顶部总开关。后续修改需点击「应用更改」。
3. 在浏览器或其他应用设置 HTTP / SOCKS5 代理 `127.0.0.1:18080`（按实际映射填写）。
4. 点击映射行的「测试」。只有「身份已确认」才表示本次请求同时取得出口 IP 并确认指定代理节点。可展开测试记录查看实际代理链。
5. 关闭总开关会恢复原全局脚本并移除 Hermit 监听，保留映射。重新启用无需重填。

「环境与恢复」提供环境诊断和退出按钮。退出管理进程、关闭浏览器标签页均不会停用已应用的固定端口；如需停用，请先关闭总开关。Clash 重启或订阅重新生成时，全局脚本会再次生成映射；订阅删除了目标时，该映射不会回退其他出口，界面显示异常，需要重新选择节点。

当前为本地构建、临时签名，未做 Developer ID 签名 / Apple 公证。跨机器分发前需要正式签名和公证；无需关闭系统安全设置。

## 源码启动与 CLI

```sh
python3 -m venv .venv
.venv/bin/python -m pip install -r requirements.lock
.venv/bin/python -m pip install -e . --no-deps
.venv/bin/hermit doctor
sh scripts/build.sh
open dist/Hermit.app
```

```sh
.venv/bin/hermit nodes
.venv/bin/hermit map add --port 18080 --target '实际节点名'
.venv/bin/hermit enable
.venv/bin/hermit status
.venv/bin/hermit map list
.venv/bin/hermit test 映射ID
.venv/bin/hermit map update 映射ID --port 18082 --target '实际节点名'
.venv/bin/hermit apply
.venv/bin/hermit disable
.venv/bin/hermit map delete 映射ID
```

手动 URI 从标准输入读取，避免进入进程参数。终端可用 `getpass` 隐藏输入：

```sh
.venv/bin/python -c 'import getpass; print(getpass.getpass("VLESS URI: "))' | .venv/bin/hermit map add --port 18081 --uri-stdin
.venv/bin/hermit map update 映射ID --port 18082 --keep-uri
```

`map add/update/delete` 只保存；`apply` 在总开关已开启时应用最新映射，关闭时保持关闭；`enable` 开启并应用。`--disabled` 可单独停用一个映射。全局参数 `--data-dir`、`--clash-dir`、`--clash-app` 应置于子命令前。MVP 只支持 macOS 的本地 Unix socket，不连接远程控制接口。

## 凭据与恢复

- 状态默认保存至 `~/Library/Application Support/Hermit/`。目录权限 0700，文件权限 0600；手动 URI 和原脚本 / 运行配置恢复快照均经 Fernet 加密，随机主密钥保存在 macOS 钥匙串。
- 供 Clash 执行的全局脚本必须包含可供内核读取的手动节点凭据；Hermit 设置其权限为 0600。Clash 生成的运行配置和 Clash 自身的备份也可能含节点凭据，受 Clash 自身的存储机制管理。不要分享这些文件或整个数据目录。
- 测试历史仅保留最近 100 条摘要，包括时间、端口、节点名、出口 IP、耗时、代理链和诊断；不保存原始 URI。查询出口 IP 会向 `api.ipify.org` 发送 HTTPS 请求。
- 写入前进行离线脚本组合与 Mihomo `-t` 校验，再建立加密事务日志。原子写入后重载内核，检查 127.0.0.1 上的目标端口；重载前后的手动策略组选择会保留。监听检查仅证明入口可用，指定节点身份以本次测试连接的 Mihomo 代理链为准。
- 操作失败时尽力恢复原脚本和运行配置。若中断或回滚失败，界面提示恢复，CLI 可执行 `.venv/bin/hermit recover`。
- 原脚本、活动订阅或生成配置在操作期间发生外部变化时拒绝覆盖，恢复记录会保留。出现此类冲突时先保留数据目录和钥匙串，再核对 Clash 的最新脚本与配置，不能删除恢复日志来绕过检查。Hermit 不提供强制覆盖开关。
- 不要删除钥匙串主密钥；已存在加密数据但密钥缺失时拒绝创建替代密钥。卸载前先停用映射并确认原脚本已恢复。

## 协议与兼容范围

活动节点直接来自 Mihomo 的 `/proxies`，排除策略组、内置 DIRECT / REJECT / PASS 等节点。手动 URI 支持 `vless://`，TCP / TLS / REALITY / WebSocket / gRPC；不支持的协议、参数、重复字段及不匹配组合明确报错。MVP 的 `encryption` 仅支持 `none`，gRPC 仅支持 `mode=gun`，TLS Vision 仅支持 TCP。不会静默忽略未知参数。

已实测 Clash Verge Rev **2.5.8** + Mihomo **v1.19.32**，服务模式通过实际进程发现 socket。自动接管还要求原脚本可同步执行且返回配置对象；不兼容时保留原文件并拒绝接管。脚本包装使用受限 QuickJS 离线检查，Clash 实际执行引擎仍由 Clash 自身决定；复杂脚本可能因运行时差异而被拒绝。每次订阅更新后应检查状态或测试，特别是存在订阅级脚本或代理集合时。

本轮公网验证覆盖一个实际 VLESS 节点的活动引用及 URI 注入两种来源。其他传输组合有解析和内核配置校验覆盖，未宣称跨全部服务器实现互通。其他 Clash 版本、Intel Mac、签名公证、菜单栏及 P1 体验增强暂未验收。

## 开发验证与打包

```sh
.venv/bin/python -m pytest -q
sh scripts/build.sh
```

`requirements.lock` 固定本轮构建使用的依赖，`Hermit.spec` 定义应用打包。安全边界、回滚、恢复、URI、脚本兼容和 HTTP API 均有自动化检查；浏览器界面和真实代理链路的验收范围见 [PLAN.md](PLAN.md)。

技术依据：[Mihomo listener 字段](https://wiki.metacubex.one/config/inbound/listeners/)、[Mihomo 控制 API](https://wiki.metacubex.one/api/)、[VLESS](https://wiki.metacubex.one/config/proxies/vless/)、[Clash Verge Rev 2.5.8 源码](https://github.com/clash-verge-rev/clash-verge-rev/tree/v2.5.8)。
