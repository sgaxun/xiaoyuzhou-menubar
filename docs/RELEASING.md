# 版本发布与更新签名

发布目标为 `sgaxun/xiaoyuzhou-player`。发布脚本生成并校验安装包和签名更新包，上传操作仅创建或更新 GitHub Release 草稿，正式发布需在 GitHub 确认。

## 旧版客户端兼容

仓库由 `sgaxun/xiaoyuzhou-menubar` 更名为 `sgaxun/xiaoyuzhou-player`。已发布的旧版客户端仍使用原更新地址，通过 GitHub 重定向获取 Releases 和更新包；新构建使用当前仓库地址。

不要重新创建同账号下的 `xiaoyuzhou-menubar` 仓库，否则旧地址的重定向会失效。保留应用标识、公钥和本地数据目录；无需重签历史更新包，也不要修改历史 manifest 中的归档地址。

## 一次性准备

1. 在项目外生成并妥善备份 MyGo 更新密钥。使用已安装的 MyGo v0.8.3：
   ```sh
   mygo keygen -o "$HOME/.config/xiaoyuzhou-menubar/update-keys"
   ```
   `.key` 是私钥，不提交到仓库、不贴到日志；`.pub` 是可提交的公钥。此更新签名与 Apple Developer ID 签名/公证不同。
2. 当前更新源已配置。仅在更换仓库或公钥时运行配置命令：
   ```sh
   npm run configure:updates -- OWNER/REPO
   # 也支持完整 https://github.com/OWNER/REPO URL 和可选 --public-key /path/to/mygo-update.pub
   ```
   默认只读取 `~/.config/xiaoyuzhou-menubar/update-keys/mygo-update.pub`。只有执行时才写 `mygo.json`：保留其他配置，设置 `updates.github`、Base64 `publicKey`、`deltas: 0`，清除互斥的 URL/S3、私钥路径和自定义标签前缀。仅接受 `.pub` 文件；不会读取私钥。
3. `npm run release` 会调用打包脚本，将当前平台版本的安装包、更新归档和 manifest 放入 `release/`，并校验版本、下载地址、大小和签名。
4. GitHub 仓库 Actions secrets 添加 `MYGO_UPDATER_PRIVATE_KEY`，内容是 MyGo `.key` 的 Base64 文本。通过 GitHub 设置或 `gh secret set MYGO_UPDATER_PRIVATE_KEY < /path/to/mygo-update.key` 设置，不把值放命令参数。

## 本地构建（默认无远端写入）

要求 Apple Silicon macOS（Xcode 命令行工具）或 Windows x64（NSIS 可由 MyGo 自动下载）、Node 20+、项目 go.mod 要求的 Go。发布版本使用应用版本号；`mygo.json` 与 `package.json` 的版本必须一致。

```sh
npm ci
npm test
MYGO_UPDATER_PRIVATE_KEY_FILE="$HOME/.config/xiaoyuzhou-menubar/update-keys/mygo-update.key" npm run release
```

也支持由密钥管理器注入 `MYGO_UPDATER_PRIVATE_KEY`；不能同时设置两种密钥来源。release 校验公私钥匹配后调用 `npm run package`，不自行再做一次 MyGo 构建。私钥只传给打包子进程，不传给 gh，不输出私钥值或敏感子进程错误。

MyGo v0.8.3 的产物契约（已对照本机模块源码 `cmd/mygo/updates.go`、`internal/update/update.go` 与 `docs/updates.md`）：

- macOS DMG：可以保留 package 脚本的人类可读名称；`release/` 必须恰有一个当前版本非空 DMG。
- 更新归档：`<MyGo-slug>-<version>-darwin-arm64.tar.gz`，必须保留原名，不能改成 DMG 命名。
- Windows 安装包：`<MyGo-slug>-setup-<version>-amd64.exe`；附带 `install.ps1`（在线下载最新安装包的引导脚本）。
- Windows 更新归档：`<MyGo-slug>-<version>-windows-amd64.tar.gz`，不是 ZIP 或安装包，保留原名。
- Manifest：按平台固定为 `update-darwin-arm64.json` 或 `update-windows-amd64.json`，包含版本、归档 URL、size 和 signature；Ed25519 签的是归档 SHA-256 摘要。不是单独 `.sig` 文件。

release 会核对 manifest 指向当前仓库 `releases/download/v<version>/`、当前版本和当前平台归档，并校验大小及签名。缺少产物或签名错误即失败，不上传。`deltas: 0` 禁用增量更新。

## 创建草稿

先提交版本及配置，并推送对应 `v<version>` 标签。脚本要求远端标签已存在，不替你建标签。配置中的仓库是唯一上传目标；有 `GITHUB_REPOSITORY` 时必须匹配，防止 fork 或错误仓库上传。

```sh
MYGO_UPDATER_PRIVATE_KEY_FILE="$HOME/.config/xiaoyuzhou-menubar/update-keys/mygo-update.key" npm run release -- --draft
```

需 gh 已登录或有 GH_TOKEN。仅创建/复用草稿并上传当前平台的产物（macOS 三个，Windows 四个）；重跑可覆盖草稿同名附件，已正式发布的版本一律拒绝覆盖。预发布版本会给草稿加 prerelease 标记。脚本从不正式发布。

## GitHub Actions

推送 `v*` 标签触发；也可在 Actions 手动运行，但必须选择与配置版本一致的标签（选择分支会失败）。矩阵使用 `macos-latest`（显式检查 arm64）与 `windows-latest`（x64），顺序为仓库/版本检查、`npm ci`、`npm test`、`npm run release -- --draft`，Go 从 go.mod 安装。签名 secret 仅传给构建发布步骤，产物另存为各平台 Actions artifact。并发按 ref 串行化，平台也串行上传，避免同时创建草稿。一个平台失败可能留下不完整草稿，必须补齐并确认两个平台 job 全部成功后才可手动发布。

本地 Windows PowerShell：

```powershell
npm ci
npm test
$env:MYGO_UPDATER_PRIVATE_KEY_FILE = "$HOME/.config/xiaoyuzhou-menubar/update-keys/mygo-update.key"
npm run release
# 仅在明确准备上传草稿时：npm run release -- --draft
```

同一草稿应包含两个平台共七个附件。各平台打包只清理自身旧安装包及更新文件，不删除另一平台 release 产物。构建临时目录 build 会重建，不能并行在同一工作区打包。

Windows 安装为每用户安装，MyGo 更新直接验证并替换归档内应用文件，不运行 NSIS 安装包；重启使用原 EXE 路径，下次启动清理仍被占用的旧文件。`install.ps1` 用于初次安装，不是应用内更新协议。以上对照 v0.8.3 的 `cmd/mygo/build.go`、`nsis.go`、`updates.go`、`updater.go` 和 `internal/update` 源码。更新 Ed25519 签名不等于 Windows Authenticode 签名，目前配置不提供 Authenticode 签名，可能出现 SmartScreen 提示。

工作流只产出草稿。人工检查两平台安装、托盘、音频、分享，以及上一版应用下载并重启安装更新，再在 GitHub Releases 页面正式发布。Windows 需要 WebView2 Evergreen Runtime。草稿不会被已安装应用作为正式更新发现。当前 package 构建跳过 Apple 公证时，不代表具备 Developer ID 分发资质；MyGo 更新签名本身不替代系统代码签名。

公钥必须从第一版开始随应用分发，后续保持稳定；丢失私钥会导致旧客户端不能信任新密钥。不要在常规发版时重新生成密钥。
