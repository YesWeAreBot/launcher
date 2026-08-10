# YesImBot Launcher

独立于 Koishi 插件系统的 CLI：初始化 Koishi App（`yesimbot init`），并管理 Koishi/YesImBot 子进程（`start` / `stop` / `status`）。

## 构建

```bash
make build          # 产物 ./yesimbot（静态、去符号，约 4MB）
make dist           # 额外用 UPX 压缩（约 1.6MB，需安装 upx-ucl）
# 或
go build -o yesimbot .
```

## 使用

```text
yesimbot init [directory] [--local <path>] [--build]
yesimbot start [--daemon] [--app <directory>]
yesimbot stop [--app <directory>]
yesimbot status [--app <directory>]
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
