# YesImBot Launcher

独立于 Koishi 插件系统的 CLI：初始化 Koishi App（`yesimbot-cli init`），并管理 Koishi/YesImBot 子进程（`start` / `stop` / `status`）。

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
```

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
