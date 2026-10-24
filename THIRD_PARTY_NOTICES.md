# 第三方依赖与许可状态

**Hermit 的根许可证尚未确定。** 本文件是依赖识别与发布准备说明，不为项目代码提供授权，也不代替每个依赖自身的许可文本。

当前 Go 代码直接导入 `github.com/metacubex/mihomo/common/convert`（v1.19.3）。该版本包含 [GNU GPL v3 许可](https://github.com/MetaCubeX/mihomo/blob/v1.19.3/LICENSE)。分发组合程序时必须处理相关源码与授权义务；不能仅添加 MIT 文件就认为覆盖了依赖。参考 [GNU 对链接组合程序的说明](https://www.gnu.org/licenses/gpl-faq.en.html#GPLStaticVsDynamic)。最终方案由维护者确认。

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

`node scripts/dependency-inventory.mjs` 会在 `bin/release-metadata/` 生成版本清单，供维护者确认依赖范围；它不是完整的 SPDX SBOM 或法律审计报告。正式发布还需汇集适用的版权声明、许可证全文与对应源码，见 [发布流程](docs/releasing.md)。

Logo 由图像生成工具依据项目需求生成并由维护者选定，来源提示词记录在 `assets/README.md`；项目尚未为品牌素材另行授予授权，也未完成商标冲突检索。
