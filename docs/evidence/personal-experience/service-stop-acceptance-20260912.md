# M82：退出请求持久化接受记录

日期：2026-09-12。规格 §3.11.44；新增 local-service-stop-acceptance/v1，记录实现位于 internal/state/service_stop.go。本地候选基于 274ed97，尚未提交。

记录绑定 boot_id、目录、完整签名请求的规范 SHA-256、首次接受时间和服务签名。持主 Writer 验证请求后排他发布 service-stop-<boot_id>.json；同请求可幂等复用，不同请求、未知内容或签名不匹配不能覆盖。只读读取核对当前目录归属，不创建/修复。

## 验证

- Go 全量/vet、localcontrol race（1.053 秒）、状态包 race（4.141 秒）通过。
- 无 Writer、过期请求、另一目录写入、复制记录到另一目录后读取、不同有效请求覆盖、未知文件覆盖均拒绝；同请求保留首次接受时间。
- 初次测试发现 canon.Marshal 不接受结构体，已改为明确字段 map，包含请求 signature；重跑通过。
- Go/Python 共用接受记录，独立核对完整请求规范摘要、boot/目录关联、接受时间范围、Ed25519 签名；164 项合同测试和 Ruff 通过。
- linux/arm64、linux/amd64、darwin/arm64、windows/amd64 的 go build ./... 通过。

## 后续接入

记录只证明接受，不证明服务已经排空或退出。下一步将 RecordServiceStopAcceptance 作为 localcontrol.Accept 的必要成功回调，接上受限 HTTP 端点与现有 drainingHandler；不得在记录失败时发送退出通知。最终停止仍需当前运行身份、排空结果与 Writer 释放证据。

当前记录方法尚未接入服务请求路径，没有新增可用退出命令，不提升 Windows 或跨系统原生验收状态。功能增量未提交、推送或合并。
