# MyGo v0.8.3 更新发布

当前尚未确定 GitHub 仓库地址，也未初始化 Git。这里不预填地址、不初始化 Git、不创建远端。待真实公共仓库确定后再配置；安装端需要能匿名访问 Releases。

## 一次性准备

1. 在项目外生成并妥善备份 MyGo 更新密钥。使用已安装的 MyGo v0.8.3：
   ```sh
   mygo keygen -o "$HOME/.config/xiaoyuzhou-menubar/update-keys"
   ```
   `.key` 是私钥，不提交到仓库、不贴到日志；`.pub` 是可提交的公钥。此更新签名与 Apple Developer ID 签名/公证不同。
2. 拿到真实仓库地址后，才手动运行下面命令（`OWNER/REPO` 是待替换参数，不是实际地址）：
   ```sh
   node scripts/configure-updates.mjs OWNER/REPO
   # 也支持完整 https://github.com/OWNER/REPO URL 和可选 --public-key /path/to/mygo-update.pub
   ```
   默认只读取 `~/.config/xiaoyuzhou-menubar/update-keys/mygo-update.pub`。只有执行时才写 `mygo.json`：保留其他配置，设置 `updates.github`、Base64 `publicKey`、`deltas: 0`，清除互斥的 URL/S3、私钥路径和自定义标签前缀。仅接受 `.pub` 文件；不会读取私钥。
3. 由负责 `package.json` 的人加入 `"release": "node scripts/release.mjs"`（可另加 `"configure-updates": "node scripts/configure-updates.mjs"`）。本次四文件修改不更改 package.json。
4. `scripts/package.mjs` 需将 MyGo 生成的当前版本 DMG、更新归档、manifest 复制到 `release/`。构建不得传 `-upload`；不得记录签名环境变量。release 脚本负责把文件型私钥转为打包子进程的 `MYGO_UPDATER_PRIVATE_KEY`，不把私钥写回配置。package 脚本也可自行支持文件型密钥，但无需重复生成归档或签名。
5. GitHub 仓库 Actions secrets 添加 `MYGO_UPDATER_PRIVATE_KEY`，内容是 MyGo `.key` 的 Base64 文本。通过 GitHub 设置或 `gh secret set MYGO_UPDATER_PRIVATE_KEY < /path/to/mygo-update.key` 设置，不把值放命令参数。

## 本地构建（默认无远端写入）

要求 Apple Silicon macOS、Node 20+、项目 go.mod 要求的 Go、Xcode 命令行工具。版本为应用版本（当前 0.2.0），不是 MyGo 的 0.8.3；`mygo.json` 与 `package.json` 的版本必须一致。

```sh
npm ci
npm test
MYGO_UPDATER_PRIVATE_KEY_FILE="$HOME/.config/xiaoyuzhou-menubar/update-keys/mygo-update.key" npm run release
```

也支持由密钥管理器注入 `MYGO_UPDATER_PRIVATE_KEY`；不能同时设置两种密钥来源。release 校验公私钥匹配后调用 `npm run package`，不自行再做一次 MyGo 构建。私钥只传给打包子进程，不传给 gh，不输出私钥值或敏感子进程错误。

MyGo v0.8.3 的产物契约（已对照本机模块源码 `cmd/mygo/updates.go`、`internal/update/update.go` 与 `docs/updates.md`）：

- DMG：可以保留 package 脚本的人类可读名称；`release/` 必须恰有一个当前版本非空 DMG。
- 更新归档：`<MyGo-slug>-<version>-darwin-arm64.tar.gz`，必须保留原名，不能改成 DMG 命名。
- Manifest：固定 `update-darwin-arm64.json`，包含版本、归档 URL、size 和 signature；Ed25519 签的是归档 SHA-256 摘要。不是单独 `.sig` 文件。

release 会核对 manifest 指向当前仓库 `releases/download/v<version>/`、当前版本和 arm64 归档，并校验大小及签名。缺少产物或签名错误即失败，不上传。`deltas: 0` 禁用增量更新。

## 创建草稿

先在真实仓库提交版本及配置，并推送对应 `v<version>` 标签。脚本要求远端标签已存在，不替你建标签。配置中的仓库是唯一上传目标；有 `GITHUB_REPOSITORY` 时必须匹配，防止 fork 或错误仓库上传。

```sh
MYGO_UPDATER_PRIVATE_KEY_FILE="$HOME/.config/xiaoyuzhou-menubar/update-keys/mygo-update.key" npm run release -- --draft
```

需 gh 已登录或有 GH_TOKEN。仅创建/复用草稿并上传三个产物；重跑可覆盖草稿同名附件，已正式发布的版本一律拒绝覆盖。预发布版本会给草稿加 prerelease 标记。脚本从不正式发布。

## GitHub Actions

推送 `v*` 标签触发；也可在 Actions 手动运行，但必须选择与配置版本一致的标签（选择分支会失败）。`macos-latest` 必须提供 arm64，工作流会显式检查。顺序为仓库/版本检查、`npm ci`、`npm test`、`npm run release -- --draft`，Go 从 go.mod 安装。签名 secret 仅传给构建发布步骤，产物另存为 Actions artifact。并发按 ref 串行化。

工作流只产出草稿。人工检查 DMG 可安装启动、manifest/归档齐全，并用上一版应用验证更新后，再在 GitHub Releases 页面正式发布。草稿不会被已安装应用作为正式更新发现。当前 package 构建跳过 Apple 公证时，不代表具备 Developer ID 分发资质；MyGo 更新签名本身不替代 macOS 签名/公证。

公钥必须从第一版开始随应用分发，后续保持稳定；丢失私钥会导致旧客户端不能信任新密钥。不要在常规发版时重新生成密钥。
