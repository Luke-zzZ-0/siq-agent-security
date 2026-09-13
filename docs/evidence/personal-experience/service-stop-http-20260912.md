# M83：本机退出 HTTP 与排空联动

日期：2026-09-12。规格 §3.11.45，候选基于 274ed97 的本地增量。

新增本机 challenge/stop 端点，继承 loopback/Host/Origin 防护；要求 CLI 标头，拒绝浏览器来源/Fetch Metadata、Authorization 和 Cookie。严格限长 JSON 拒绝重复、缺失、未知字段和尾随内容。验证当前运行签名后，由持主 Writer 的服务记录接受，再返回 202 并通知既有停止 channel。复用 drainingHandler，不提前释放仍在工作的请求所用 Writer。

## 验证

- Go 全量/vet、server race（27.334 秒）、CLI race（7.688 秒）及四目标构建通过。
- HTTP 定向测试：挑战验签、接受记录在通知前可读、签名响应、重放拒绝；写入目标受阻时 503 且不通知，解除阻碍后同请求可成功重试。
- 鉴权分离和请求负向：Authorization/Cookie/Origin/Fetch Metadata 拒绝；未知/重复/null 字段、尾随 JSON、超限、非对象和错误签名拒绝。
- 隔离 Linux arm64 原生子进程测试：最终候选 serve 使用临时状态目录和随机 loopback 端口，获取挑战、用对应本机密钥签署请求，HTTP 返回 202，真实进程正常退出，接受记录与响应一致，主 Writer 可重新获取。测试 0.104 秒。没有对生产实例发送停止请求。

原生复验命令（从 apps/agentshield 运行）：

```bash
SIQ_TEST_SERVE_STOP=1 SIQ_TEST_BINARY=/tmp/siq-m83-build/siq-linux-arm64 \
  go test ./cmd/agentshield -run TestNativeServeSignedStop -count=1
```

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 09b12b3f3a9bd1ffbe57f6bae2141333b144d5b6f047df836db6c5341da1a8ec |
| linux/amd64 | 3d09d0abe630078834aed8ea02f08e47fb79321e1a27993b55116ac05acf19ff |
| darwin/arm64 | f5293b9482574a9c0969bd75f290e0039bd18ba8d69e4ab27a62f92c6318d4a8 |
| windows/amd64 | 1c924fef756cc90d6ea8715fd19e07f6cfe0a10de1f2ff1d4d17b8c9f3557006 |

## 边界

202 只是接受，不是退出完成；客户端还需验证当前运行、退出结果及 Writer。原生证据仅 Linux，Windows Task Scheduler/PowerShell/用户会话的退出路径未实测。尚无用户级停止 CLI，也未接 Windows task-stop、注销或 setup/teardown。下一步接本机签名停止客户端与最终确认，不能把上述子进程实验当作跨 OS 支持完成。

功能增量仍仅本地，未提交、推送或合并远端。
