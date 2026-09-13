# M86：停止完成确认与恢复 CLI

日期：2026-09-12。规格 §3.11.48，`stop --confirm-stop [--recover <boot_id>]`，本地候选基于 274ed97。

默认请求签名退出并等待本次结果；recover 仅读取历史接受记录，不发送新请求。等待本次 drained 并取得主 Writer 后，持锁复验接受/结果一致，释放临时 Writer 后才输出结果。缺失/忙可等待，drain_failed、篡改、冲突与异常拒绝。超时带原 boot_id 提示，记录保留。

## 验证

- 六项状态测试：已释放、Writer 忙、结果缺失、排空失败、结果篡改、接受记录不匹配。只有已释放场景通过；成功无锁泄漏，缺失/忙给出恢复身份。确认参数负向通过。
- Go 全量/vet/CLI race（6.289 秒）、四目标构建通过。
- 最终候选隔离 Linux：直接 HTTP、stop-request CLI、stop CLI 三条真实退出旅程通过（共 0.415 秒）；stop 返回时结果已核对。服务退出后再调用完整 stop --recover，返回相同结果，无需在线服务。每条路径核对退出码、接受记录、排空签名和主 Writer 可用性。

```bash
SIQ_TEST_SERVE_STOP=1 SIQ_TEST_BINARY=/tmp/siq-m86-build/siq-linux-arm64 \
  go test ./cmd/agentshield -run 'TestNative(ServeSignedStop|StopRequestCLI|StopCLI)' -count=1
```

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | e410e42bc079f1976c3b5b7c6f02686835111d0ade2879536eda000fb5ad3f99 |
| linux/amd64 | b809f107a40d2c21c94a1e90d1262f909fef317917af9eee21df8aa1cb2fa05b |
| darwin/arm64 | 7a213451f111f8b1a93f3033f2d163b708de763b28fa8d6ced5e851aaf5ccb14 |
| windows/amd64 | a41042edeb089feaf9e90a58ddae2d157a949cd5aeb4acce86839e0c1e36f058 |

## 边界与后续

确认是当前目录在检查时已排空且没有活动 Writer，不阻止系统管理器之后重新启动。recover 不会停止新实例；若新实例持 Writer，则旧 drained 记录不能通过完成检查。Windows task-stop 还需任务归属与运行状态前后复验，本批没有 Windows/macOS 原生停止证据。下一步接 Windows 停止/注销与 setup/teardown，UX-003 仍未完成。功能增量本地保留，未提交或推送。
