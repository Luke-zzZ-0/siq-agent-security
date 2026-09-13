# M87：Windows 任务正常停止编排

日期：2026-09-12。规格 §3.11.49。本地候选基于 274ed97，功能未提交或推送。

`task-stop --confirm-stop` 将已验证的任务运行状态与本机签名停止协议串联。运行实例需要本次 drained 记录、主 Writer 释放、系统任务 ready/0 实例/零返回码；最终持主 Writer 复验完整配置、运行状态和签名记录。初始空闲仅确认空闲，不要求历史返回码为零。排队拒绝，不强制结束任务，不注销配置。

## 验证

- `go test ./cmd/agentshield -run TestWindowsTaskStop -count=1`：10 项模拟编排及确认参数负向通过。覆盖正常停止、原本空闲、排队、未知配置、请求失败、排空失败、仍在运行、退出失败、最终配置漂移、写锁占用；核对请求次数、归属记录保留和验证锁释放。
- `go vet ./... && go test ./... && go test -race ./cmd/agentshield` 通过，CLI race 6.954 秒。
- 四目标构建通过。

| 目标 | SHA-256 |
| --- | --- |
| linux/arm64 | 9129d5923d353d464e4cfd74cbd1527ea747050b66967064f17c191a64bd5099 |
| linux/amd64 | ca28679a37b27bc3213fb69354e6be1f20fd10a7f5b250aacc672fc093adf1b8 |
| darwin/arm64 | 98580400fd91d4b78c90a254cf09678066c2b64926c949c5f35cdf5a424ca164 |
| windows/amd64 | 6e2faeec8e29a1cccf973ee9e4587eb25f2f3766aebf50249c53127ef159d862 |

## 边界

使用临时目录中的真实签名记录与 Writer；系统查询、状态和请求入口使用测试回调。没有 Windows 宿主，未执行 PowerShell/COM，不将测试或编译计为原生支持。检查只确认当时空闲，不阻止之后再次启动。Windows 注销与 setup/teardown 仍待开发，UX-003 不标完成。
