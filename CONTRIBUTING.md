# 开发与构建

需要 Apple Silicon Mac、Go 1.27.3、Node 24 和 Xcode Command Line Tools。修改 Icon Composer 图标时需要完整 Xcode。

## 构建 macOS 应用

```sh
git clone https://github.com/ruipenglu/Hermit.git
cd Hermit
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.34
export PATH="$(go env GOPATH)/bin:$PATH"
wails3 package
open bin/hermit.app
```

产物位于 `bin/hermit.app`，使用临时签名，未经过 Apple 公证。Wails CLI、Go 模块与 npm runtime 保持相同版本。

## 开发与测试

```sh
npm ci --prefix frontend
npm run dev --prefix frontend   # 浏览器示例数据预览
npm test --prefix frontend
go test -race ./internal/...
```

浏览器预览默认在 `http://127.0.0.1:9245/`。`wails3 dev` 则运行真实后端。修改 Go 服务后，用 `wails3 generate bindings -clean=true -ts -i` 更新绑定。

核心测试使用临时目录、假控制器和测试密钥。仅设置数据目录环境变量不能完全隔离 Clash，不要在测试中读取或修改真实订阅、运行配置和钥匙串。

反馈或 PR 说明问题及验证结果即可；不上传真实凭据。自有代码采用 [MIT](LICENSE)，第三方代码保留原许可。发布方法见 [发布说明](docs/releasing.md)。
