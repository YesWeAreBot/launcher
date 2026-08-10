# YesImBot Launcher

独立于 Koishi 插件系统的 CLI：初始化 Koishi App（`yesimbot-cli init`），并管理 Koishi/YesImBot 子进程（`start` / `stop` / `status`）。

## 一键安装

从 GitHub Release 下载当前平台的预构建包，并安装到用户目录：

```bash
# Linux / WSL / macOS
curl -fsSL https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.sh | sh
```

```powershell
# Windows PowerShell
irm https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.ps1 | iex
```

默认从 `nightly` Release 安装。需要指定频道或安装目录时，可下载脚本后执行：

```bash
sh install.sh --channel nightly --install-dir "$HOME/.local/bin"
```

```powershell
.\install.ps1 -Channel nightly -InstallDir "$env:LOCALAPPDATA\YesImBot\bin"
```

Linux、WSL 和 macOS 默认安装到 `~/.local/bin`；Windows 默认安装到 `%LOCALAPPDATA%\YesImBot\bin`。如果安装目录已经在 PATH 中，脚本不会重复写入；新增配置在重新打开终端后生效。安装完成后脚本会直接输出 `yesimbot-cli --help`。

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
yesimbot-cli start [--daemon] [--app <directory>]
yesimbot-cli stop [--app <directory>]
yesimbot-cli status [--app <directory>]
yesimbot-cli uninstall [directory] [--app <directory>] [--keep-app] [--yes]
```

`uninstall` 默认停止实例并把整个 Koishi App 移动到同级备份目录，保证可逆；需要保留 Koishi App 时使用 `--keep-app`。

## 目录

```text
main.go      # 入口
cmd/         # 命令定义（init/start/stop/status）
internal/    # 内部依赖：路径、命令执行、配置生成、init 编排、运行时
```

## 测试

```bash
make test
```
