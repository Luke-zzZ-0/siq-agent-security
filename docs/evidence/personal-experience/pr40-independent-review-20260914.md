# PR #40 独立合并复核（2026-09-14）

受审源：973541733ccdb25d4578e56033901e4829ff257c，基线 main aac1154bd0fe25d38f451716f1b1edd385f6358a。本补充提交只修正启动器回归测试，不改运行时代码或历史 Windows 证据。

## 发现与修正

Linux ARM64 原生启动器测试在 alias 路径处失败：状态存储已拒绝符号链接目录，而旧测试期待直接传入 alias 能复用。该期待在 main 已存在，并非本次 Windows 实现引入。保留符号链接屏障，测试改为断言拒绝、原实例记录不变、链接保留、没有新增进程、现有进程仍运行；再以解析后的规范路径验证正常复用（与启动器 CLI 的路径处理一致）。后续配对、停止、重启断言全部执行。

## 验证

- 本机 Linux ARM64，受审源构建的真实二进制：`SIQ_TEST_BINARY=/tmp/siq-pr40-linux-arm64 python3 -m pytest scripts/personal-experience/test_start_local.py -q`，修正前 13 passed / 1 failed，修正后 14 passed / 0 skipped。
- 同源 `go vet ./...`、`go test ./...` 全部成功，gofmt 无输出。
- CGO_ENABLED=0 构建 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全部成功。
- 新增负向 XML 测试保留未知元素、非默认值、提权、重复节点的拒绝；旧签名源不得由新模板覆盖。
- Windows 原生证据仍以 sunbo 的 ebc472f 候选为准，本机不重复声明 Windows 实测。

## 合并边界

接受 Windows 构建入口、任务 XML/COM 传输与精确归属读回、状态诊断及阶段性证据；Windows 全量 Go 的 16 个失败包、两包超时与 Issue #39 仍未关闭。OpenClaw/Hermes/WorkBuddy 的 Windows 全旅程、macOS、N09 总体验收均不因此完成。历史证据保留原字节、原摘要及原候选，最终 PR 还须通过其最终提交的 CI。
