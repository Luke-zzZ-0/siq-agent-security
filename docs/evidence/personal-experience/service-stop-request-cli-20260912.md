# M84：本机签名退出请求 CLI

日期：2026-09-12。规格 §3.11.46；`stop-request --confirm-stop`，本地后继分支基于 274ed97。

客户端使用既有状态目录/配置端口与私钥，持生命周期锁，先检查目录健康，再获取、验签并签署当前运行挑战。使用无代理/无重定向客户端，不发送 bearer 或 Cookie。202 响应必须与本机签名接受记录完全匹配；POST 传输失败时可从本机记录确认同一请求已接受，不自动重发。输出接受记录 JSON，语义明确为请求已接受。

## 验证

- 六项 HTTP 模拟：接受成功、响应丢失但记录存在、响应有接受但无记录、挑战伪造、响应与记录冲突、服务拒绝；失败挑战不发送 stop，其他每次只发一个请求。
- 明确确认、响应限长/未知字段/尾随内容检查通过。
- Go 全量/vet/CLI race（6.404 秒）、四目标构建通过。
- 隔离 Linux 最终候选：既有直接 HTTP 停止和新增真实 CLI 停止两条路径共 0.197 秒。各自使用独立临时目录/随机端口，真实 serve 子进程退出码为零，签名接受记录与响应一致，主 Writer 可重新获取。CLI 路径调用最终二进制 stop-request，不只调用 Go helper。

复验命令：

```bash
SIQ_TEST_SERVE_STOP=1 SIQ_TEST_BINARY=/tmp/siq-m84-build/siq-linux-arm64 \
  go test ./cmd/agentshield -run 'TestNative(ServeSignedStop|StopRequestCLI)' -count=1
```

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | df7921398e5334f700f025056cf6d3aa1f6d74f32556c42d24c44bdc4a3e143d |
| linux/amd64 | a8aa1819c97ae389bd71cbd21626b672cd8c14fd41d9ed2f7e866524a3d91808 |
| darwin/arm64 | c9e4e2d906cc187935d1c951e73cb020a3a0bdcce5ef722f82aa9c7250b6041f |
| windows/amd64 | e775acd04d931fa3a83e7def75ae791f440f54cf05d751a8f3f2e612e0b88a5d |

## 待完成

本批 CLI 仅核对接受，不等待或宣称正常退出。下一步提供本次排空结果的持久化证明，再与 Writer/Windows 任务状态联合确认；连接中断和 202 都不能单独证明已退出。没有 Windows/macOS 原生停止证据，UX-003 保持 doing。功能仍未提交、推送或合并。
