# 开发与本地构建

## 环境

- Apple Silicon macOS，或 Windows 10/11 x64。仅支持原生构建，不把交叉编译当作安装包验证。
- Go 版本满足 `go.mod` 要求。
- Node.js 20 或更新版本及 npm，用于界面编译和构建脚本。
- macOS：Xcode Command Line Tools。
- Windows：WebView2 Evergreen Runtime（运行时），NSIS（可选预装；MyGo 找不到时自动下载，需网络）。PowerShell 中设置环境变量使用 `$env:MYGO_GO = "C:\path\to\go.exe"`。

## 构建

```sh
npm ci
npm test
npm run package
```

`npm test` 编译界面，运行 Node 组件／字幕调度测试和 Go 测试；测试使用模拟音频、隔离的临时目录及模拟网络。需要额外覆盖并发访问时，可运行 `go test -race ./...`。

`npm run package` 生成应用图标、编译界面和 macOS 原生分享助手、构建并临时签名 App，最后生成和校验 DMG。分享助手源文件位于 `native/share.swift`，使用 Xcode Command Line Tools 编译，产物不提交到仓库。

- App：`build/darwin-arm64/小宇宙播放器.app`
- 安装包：`release/小宇宙播放器-<版本>-AppleSilicon.dmg`
- Windows 程序：`build/windows-amd64/小宇宙播放器.exe`
- Windows 安装包：`release/xiaoyuzhou-player-setup-<版本>-amd64.exe`
- Windows 安装脚本：`release/install.ps1`（从配置的 GitHub Releases 下载最新安装包，不是离线安装包）。

Windows 使用同样的 `npm ci`、`npm test`、`npm run package`，不编译 Swift 分享助手，不运行 codesign/hdiutil。MyGo 显式使用 `-platform windows/amd64`，不传 `-skip-installer`，确保 NSIS 安装包实际生成。只分发安装包，不单独分发缺少附属文件的裸 EXE。无更新签名密钥时仅产出安装包和安装脚本；签名更新包见发布流程。


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

Windows 安装包默认安装到 `C:\Program Files\XiaoyuzhouPlayer`（系统 Program Files 位于其他盘时跟随系统），安装和卸载需要管理员权限。程序仍使用普通权限启动，账户与播放数据仍按用户保存在 AppData。普通权限下的更新检查不触发 UAC；用户点击安装更新时，由管理员辅助进程重新读取内置更新源、匹配版本并使用原有公钥校验归档，再写入 Program Files。旧 AppData 安装的记忆路径不会覆盖新默认位置；手动选择路径或 `/D=` 参数仍有效。
