# M116 原文仓逐任务授权管理 API

日期：2026-09-13；任务：UX-013；规格：§3.12.25、ADR-048。

本批把 M114 的签名 Grant/Revocation 权限模型接入本地管理服务。管理者可以为一个任务签发限定内容种类、采集时限、保留期和单条明文大小的授权，查询完整授权清单及单条状态，并使用预期 Grant 签名执行撤销。Grant 只持久化任务与操作者的 SHA-256 摘要；Revocation 同样只持久化撤销者摘要。两类记录均由本机 Ed25519 身份签名、不可变发布，撤销采用签名比较交换；成功撤销后既有密文保留，后续采集被拒绝。

本地管理服务新增：

- `GET /v1/raw-task-content/grants`：返回 `local-raw-task-content-grants/v1`，按 Grant ID 稳定排序，并把每条记录投影为 active、expired 或 revoked。
- `POST /v1/raw-task-content/grants`：接收 `local-raw-task-content-grant-create/v1` 严格正文，成功返回 201 和完整签名 Grant。
- `GET /v1/raw-task-content/grants/{grant_id}`：返回 `local-raw-task-content-grant-view/v1`，包含已验签 Grant、状态和可选的已验签 Revocation。
- `POST /v1/raw-task-content/grants/{grant_id}/revoke`：接收 `local-raw-task-content-revoke/v1` 严格正文，以预期 Grant 签名绑定撤销目标，成功返回不可变的签名 Revocation。

所有入口只允许配对后的管理会话，决策凭据返回 403；响应均为 `Cache-Control: no-store`。每次请求先重新读取 Activation、验签并核对独立 AES 密钥指纹。未启用返回 409，Activation 或密钥损坏返回 503。清单读取采用完整目录校验：任一 Grant/Revocation 文件损坏、孤立或超出容量时整次读取失败，不返回可能误导用户的部分结果。请求拒绝缺失、null、重复字段、未知字段、尾随 JSON、越界数值、无效标识和错误的比较交换签名。

本批没有增加原文采集 HTTP 入口。管理会话只能管理授权，不能直接写入原文；后续运行时采集仍需把短时凭据同时绑定到既有运行时身份、任务和 Grant，并在每次写入前复验权限。

验证通过：

- `go test ./...` 与 `go vet ./...`（`apps/agentshield`）。
- `go test -race ./internal/rawcontent ./internal/server -run 'Test(Authority|Activation|RawTaskContent)' -count=1`。
- Grant Create、Revoke、Grant View、Grants List 固定 Go 样例通过 Python Draft 7 合同；Grant/Revocation 固定种子样例由 Python 独立验签。合同测试共 184 项，Ruff 通过。
- `CGO_ENABLED=0` 下 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标编译通过。
- `gofmt -l` 与 `git diff --check` 无输出。

边界：运行时采集凭据和写入入口、密文清单/读取/删除、原文仓前端管理、诊断包排除检查及真实平台端到端验证仍待后续批次。四目标编译不是 macOS/Windows 原生运行证据。本批未提交、推送或合并。
