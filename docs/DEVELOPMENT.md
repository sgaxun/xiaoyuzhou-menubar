# 开发与本地构建

## 环境

- Apple Silicon macOS。
- Go 版本满足 `go.mod` 要求。
- Node.js 20 或更新版本及 npm，用于界面编译和构建脚本。
- Xcode Command Line Tools。

## 构建

```sh
npm ci
npm test
npm run package
```

`npm test` 编译界面并运行 Go 测试，测试使用隔离的临时目录及模拟网络。需要额外覆盖并发访问时，可运行 `go test -race ./...`。

`npm run package` 生成应用图标、编译界面、构建并临时签名 App，最后生成和校验 DMG。

- App：`build/darwin-arm64/小宇宙播放器.app`
- 安装包：`release/小宇宙播放器-<版本>-AppleSilicon.dmg`

若 Go 不在 PATH 中，可指定路径：

```sh
MYGO_GO=/absolute/path/to/go npm run package
```

单独构建 Go 程序前，先执行 `npm run build:ui`，生成需要嵌入的 `assets/player.html`。本地打包不要求更新签名密钥；正式发布使用[发布流程](RELEASING.md)。

## 代码结构

| 路径 | 用途 |
| --- | --- |
| `main.go`、`updates.go` | 菜单栏、窗口、系统媒体控制和更新桥接 |
| `ui/` | React 播放器、订阅动态、shownotes 和登录界面 |
| `internal/player/` | 本地数据、播放列表、订阅缓存和新单集定时检查 |
| `internal/podcast/` | 小宇宙内容读取及账号认证接口 |
| `internal/qrauth/` | 扫码登录状态机 |
| `internal/appupdate/` | 软件更新检查、下载和安装状态 |
| `scripts/` | 界面构建、测试和打包发布脚本 |

构建产物、依赖目录和个人账号数据不提交到仓库。调试时可通过 `XIAOYUZHOU_MENUBAR_DATA_DIR` 指定独立数据目录，避免影响日常使用的数据。
