# YesImBot Launcher

独立于 Koishi 插件系统的 CLI：初始化 Koishi App（`yesimbot-cli init`）、更新并重建 YesImBot source（`yesimbot-cli update`），管理 Koishi/YesImBot 子进程（`start` / `stop` / `status`），并检查或更新 CLI 自身（`self-update`）。

> 当前 v4 尚未发布到 npm，也没有上架 Koishi 插件市场。不要使用 `yarn add koishi-plugin-yesimbot` 或在插件市场搜索安装；`yesimbot-cli init` 会从 GitHub `dev` 分支获取 YesImBot 源码、构建并写入 Koishi workspace，这是当前官方安装路径。

## 一键安装

从 GitHub Release 下载当前平台的预构建包，并安装到用户目录：

```bash
# Linux / WSL / macOS
curl -fsSL https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.sh | sh
# curl -fsSL https://cdn.jsdelivr.net/gh/YesWeAreBot/launcher@main/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.ps1 | iex
# irm https://cdn.jsdelivr.net/gh/YesWeAreBot/launcher@main/install.ps1 | iex
```

默认从 `nightly` Release 安装。需要指定频道或安装目录时，可下载脚本后执行：

```bash
sh install.sh --channel nightly --install-dir "$HOME/.local/bin"
```

```powershell
.\install.ps1 -Channel nightly -InstallDir "$env:LOCALAPPDATA\YesImBot\bin"
```

Linux、WSL 和 macOS 默认安装到 `~/.local/bin`；Windows 默认安装到 `%LOCALAPPDATA%\YesImBot\bin`。如果安装目录已经在 PATH 中，脚本不会重复写入；新增配置在重新打开终端后生效。安装完成后脚本会直接输出 `yesimbot-cli --help`。

### 安装 YesImBot v4

CLI 安装完成后，运行 `yesimbot-cli init` 接入 v4。首次使用需要先准备 Node.js 18+ 和 Git：

```text
1. 安装 Node.js 18+ 与 Git
2. 安装 yesimbot-cli
3. yesimbot-cli init
4. cd yesimbot-app
5. yesimbot-cli start --daemon
6. yesimbot-cli status
```

```bash
yesimbot-cli init          # 默认创建 ./yesimbot-app，结束后按提示选择是否启动
# 以后需要再次启动时：
cd yesimbot-app
yesimbot-cli start --daemon
```

```powershell
yesimbot-cli init .\my-app # 创建到指定目录，或传入已有 Koishi App，结束后按提示选择是否启动
# 以后需要再次启动时：
cd .\my-app
yesimbot-cli start --daemon
```

`init` 会下载 Koishi boilerplate、从 GitHub `dev` 分支克隆 YesImBot 源码，构建插件并写入 workspace 依赖；当前 v4 不通过 npm 或插件市场发布。Koishi boilerplate 自带 SQLite，默认无需单独安装数据库。

如果当前目录不是 Koishi App，`start` / `stop` / `status` / `update` / `uninstall` 会自动尝试同级的 `yesimbot-app`；也可以在任意位置使用 `--app <目录>` 指定 App。

非交互式环境执行 `init` 需要显式传入 `--yes`，避免脚本在未确认时修改已有 Koishi App：

```bash
yesimbot-cli init ./my-app --yes
```

### GitHub 镜像加速

可选设置 `GITHUB_MIRROR` 为 GitHub 根地址；脚本和 CLI 会用它下载 Release 二进制、Koishi boilerplate ZIP，以及 YesImBot 源码。未设置时使用官方 GitHub，末尾 `/` 会自动处理：

```bash
GITHUB_MIRROR=https://gh-proxy.com/https://github.com sh install.sh --channel nightly
GITHUB_MIRROR=https://gh-proxy.com/https://github.com yesimbot-cli init ./my-app
```

```powershell
$env:GITHUB_MIRROR = 'https://gh-proxy.com/https://github.com'
.\install.ps1 -Channel nightly
yesimbot-cli init .\my-app
```

`init` 会优先探测镜像 Git 地址，不可达时回退官方；boilerplate ZIP 和安装脚本二进制下载失败时会直接报错。

## 构建

```bash
make build
make dist
# 或
go build -o yesimbot-cli .
```

## 使用

```text
yesimbot-cli init [directory] [--local <path>] [--build]
yesimbot-cli init [directory] --yes
yesimbot-cli start [--daemon] [--app <directory>]
yesimbot-cli stop [--app <directory>]
yesimbot-cli status [--app <directory>]
yesimbot-cli update [--app <directory>]
yesimbot-cli self-update [--check] [--channel <channel>]
yesimbot-cli uninstall [directory] [--app <directory>] [--keep-app] [--yes]
yesimbot-cli restore <backup-directory>
```

`update` 会先从 `origin/dev` 拉取最新源码，然后自动重装依赖、构建插件并刷新 App 配置；`init` 也会复用已存在的 source。
`uninstall` 默认停止实例并把整个 Koishi App 移动到同级备份目录，保证可逆；需要保留 Koishi App 时使用 `--keep-app`，此时 `package.json` 和 `koishi.yml` 会先备份到 App 内的 `.yesimbot-uninstall-*.bak` 目录。
`restore` 可以恢复默认卸载产生的整个 App 备份，也可以从 `--keep-app` 的配置备份中恢复 `package.json` 和 `koishi.yml`。
`self-update --check` 会下载并读取远端二进制版本，显示当前版本与远端版本，避免只提示“资产可用”造成误导。

## Launcher 配置

`yesimbot-cli init` 会在 Koishi App 内生成 `.yesimbot/launcher.yaml`，用于控制已发现的 YesImBot 插件默认启用状态：

```yaml
plugins:
  koishi-plugin-yesimbot-console:
    enabled: true
  koishi-plugin-yesimbot-usage:
    enabled: true
```

修改该文件后重新运行 `yesimbot-cli init`，配置会合并进 App 的 `koishi.yml`；未列出的插件保持 Launcher 默认值。

## 目录

```text
main.go      # 入口
cmd/         # 命令定义（init/update/start/stop/status）
internal/    # 内部依赖：路径、命令执行、配置生成、init 编排、运行时
```

## 测试

```bash
make test
```
