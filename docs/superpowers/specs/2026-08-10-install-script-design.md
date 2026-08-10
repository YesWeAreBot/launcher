# YesImBot Launcher 一键安装脚本设计

**日期：** 2026-08-10
**状态：** 已确认

## 目标

提供无需 Go、Python 或其他项目运行时的一键安装入口，从 GitHub Release 下载当前平台的预构建 `yesimbot-cli`，安装到用户目录，幂等配置 PATH，并在安装结束时输出 CLI help。

默认 Release 频道为 `nightly`，可通过参数覆盖。支持 Linux、WSL、macOS 和 Windows，覆盖现有发布流程提供的 amd64 与 arm64 资产。

## 范围与约束

- 新增两个原生脚本：
  - `install.sh`：Linux、WSL、macOS
  - `install.ps1`：Windows PowerShell
- 默认频道：`nightly`。
- Release 仓库固定为 `YesWeAreBot/launcher`。
- 支持覆盖频道和安装目录。
- 使用用户级安装，不要求管理员权限：
  - Unix：`~/.local/bin`
  - Windows：`%LOCALAPPDATA%/YesImBot/bin`
- 如果安装目录已经在 PATH 中，不重复写入。
- 不新增 Go、Python 或第三方安装器依赖。
- 不实现包管理器集成、后台自动更新、Release 签名或校验和验证。

## 发布资产映射

现有 nightly Release 直接发布单文件二进制，文件名为：

- `yesimbot-cli-linux-amd64`
- `yesimbot-cli-linux-arm64`
- `yesimbot-cli-darwin-amd64`
- `yesimbot-cli-darwin-arm64`
- `yesimbot-cli-windows-amd64.exe`
- `yesimbot-cli-windows-arm64.exe`

脚本根据运行平台与 CPU 架构选择资产，并从 GitHub Release 固定下载地址获取：

```text
https://github.com/YesWeAreBot/launcher/releases/download/{channel}/{asset}
```

不下载压缩包，不需要额外解压工具。

## 脚本接口

Unix 脚本：

```text
sh install.sh [--channel CHANNEL] [--install-dir PATH]
```

PowerShell 脚本：

```text
.\install.ps1 [-Channel CHANNEL] [-InstallDir PATH]
```

参数默认值分别为 `nightly` 和平台默认安装目录。

## 执行流程

1. 解析参数并校验必需值。
2. 识别 OS 与 CPU 架构；不支持的组合直接报错并以非零状态退出。
3. 映射到对应 Release 资产名。
4. 创建安装目录。
5. 将下载内容写入临时文件，下载成功后再替换目标文件，避免网络中断留下损坏程序。
6. Unix 对目标文件设置可执行权限；Windows 直接安装 `.exe`。
7. 检查安装目录是否已经存在于 PATH：
   - Unix：检查当前环境变量和将要修改的 shell 配置，只有缺失时才追加配置。
   - Windows：读取当前用户环境变量的 PATH 项，只有缺失时才通过用户级环境变量 API 追加。
8. 直接调用安装目录中的二进制执行 `--help`，不依赖当前 shell 立即刷新 PATH。
9. 输出频道、平台、架构、安装路径、PATH 处理结果和 help。

## 平台行为

### Linux、WSL、macOS

使用 `uname -s` 和 `uname -m` 识别平台。支持：

- `Linux` + `x86_64`/`amd64`
- `Linux` + `aarch64`/`arm64`
- `Darwin` + `x86_64`/`amd64`
- `Darwin` + `arm64`/`aarch64`

脚本使用 `curl`（不可用时回退到 `wget`）下载。PATH 配置按当前 shell 选择：Bash 写入 `~/.bashrc`，Zsh 写入 `~/.zshrc`，其他 shell 回退到 `~/.profile`；只有当前环境变量和目标配置都缺少安装目录时才追加一行带明确标记的配置。安装完成后提示重新打开终端或加载对应配置文件。

### Windows

PowerShell 使用 `$env:PROCESSOR_ARCHITECTURE` 识别 `AMD64` 和 `ARM64`，调用 `Invoke-WebRequest` 下载。通过 .NET 的当前用户环境变量 API 持久化 PATH，不修改系统级 PATH。当前 PowerShell 进程的 PATH 同时更新，以便随后直接调用命令；无法自动影响已经打开的其他终端。

## 失败处理

以下情况立即报错、返回非零状态，并尽量保留已有安装：

- 缺少或无法识别的平台/架构。
- Release 资产不存在或网络下载失败。
- 安装目录创建、临时文件替换或权限设置失败。
- PATH 配置失败。
- 安装后的 help 执行失败。

当前不做校验和或签名验证，因为 Release 尚未发布相应的校验文件或签名资产。未来 CI 增加这些资产后，再在下载替换前加入验证。

## README 使用方法

README 增加：

```bash
# Linux / WSL / macOS
curl -fsSL https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.sh | sh

# Windows PowerShell
irm https://raw.githubusercontent.com/YesWeAreBot/launcher/main/install.ps1 | iex
```

同时记录本地执行方式、`--channel nightly` 默认值、可选参数、用户级安装目录、Bash/Zsh/其他 shell 的 PATH 配置行为、PATH 生效提示和安装后 help 行为。

## 验证

- `sh -n install.sh`。
- 在 PowerShell 环境执行脚本解析/语法检查。
- 用临时安装目录验证参数解析、资产映射和重复运行不会重复写入 PATH。
- 对安装后的二进制执行 `--help`。
- 检查 README 示例与脚本参数一致。

不新增独立安装器 CI；跨平台实际下载验证依赖对应操作系统环境，脚本本身保留可独立运行的最小逻辑。
