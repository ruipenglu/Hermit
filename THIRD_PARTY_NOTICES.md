# 第三方依赖与许可状态

Hermit 的自有代码、文档及维护者有权授权的素材采用根目录的 [MIT 许可证](LICENSE)。第三方代码、生成代码中的第三方部分和依赖保留各自的版权与许可；MIT 不替代这些许可。

当前 Go 代码直接导入 `github.com/metacubex/mihomo/common/convert`（v1.19.3）。该版本包含 [GNU GPL v3 许可](https://github.com/MetaCubeX/mihomo/blob/v1.19.3/LICENSE)。MIT 代码可用于 GPLv3 组合程序，但包含该组件的完整应用分发仍须遵守 GPLv3，包括对应源码与许可声明等要求，不能把完整应用标为仅受 MIT 约束。Hermit 自有代码的 MIT 授权保持有效。参考 [GNU 对链接组合程序的说明](https://www.gnu.org/licenses/gpl-faq.en.html#GPLStaticVsDynamic)。

## 直接运行依赖

| 组件 | 用途 | 上游许可入口 |
| --- | --- | --- |
| Mihomo | 节点分享链接解析 | [上游许可](https://github.com/MetaCubeX/mihomo/blob/v1.19.3/LICENSE) |
| Wails v3 / runtime | 原生窗口与前后端调用 | [上游仓库](https://github.com/wailsapp/wails) |
| React / React DOM | UI | [上游许可](https://github.com/facebook/react/blob/main/LICENSE) |
| goja | JavaScript 脚本校验 | [上游许可](https://github.com/dop251/goja/blob/master/LICENSE) |
| fernet-go | 凭据与快照加密 | [上游许可](https://github.com/fernet/fernet-go/blob/master/LICENSE) |
| yaml.v3 | 配置读写 | [上游许可](https://github.com/go-yaml/yaml/blob/v3/LICENSE) |

实际版本以 `go.mod` / `go.sum` 和 `frontend/package-lock.json` 为准；开发、构建与间接依赖也需纳入正式分发审查。链接不是随包附带许可文本的替代品。

发布脚本将实际 Go 与生产 npm 依赖的许可、版权声明和对应源码随包提供。新增依赖仍需核查许可兼容性，见 [发布说明](docs/releasing.md)。

Logo 由图像生成工具依据项目需求生成并由维护者选定，来源提示词记录在 `assets/README.md`；维护者有权授权的部分随项目采用 MIT，不代表已完成商标冲突检索。
