# M118 原文记录管理读取、删除与过期清理 API

日期：2026-09-13；任务：UX-013；规格：§3.12.27、ADR-048。

本批为独立原文仓增加管理会话专用访问入口。原始任务 ID 只通过 POST 正文传入并在请求期计算摘要，不出现在 URL、响应或磁盘元数据中。决策凭据、Runtime Identity、自检凭据和匿名请求不能访问。

新增入口：

- `POST /v1/raw-task-content/records/search`：按任务返回 active/expired 记录元数据；不返回 nonce、ciphertext 或明文。
- `POST /v1/raw-task-content/records/{record_id}/read`：显式解密 active 记录，返回 `contains_plaintext=true`、记录元数据和已过滤的 path/value。
- `POST /v1/raw-task-content/records/{record_id}/delete`：要求 task_id 及与路径一致的 confirm_record_id，删除 active 或 expired 密文。
- `POST /v1/raw-task-content/purge-expired`：要求 `confirm_expired_only=true`，只删除达到到期边界的密文并返回数量与释放字节。

清单先验证完整且不超过 4096 项的内容目录，并对每条封套执行结构、时间、任务摘要、内容摘要和 AES-256-GCM 认证；任一无关任务记录损坏也会使整次清单失败。读取在报告 expired 前完成认证。单条删除同样先认证密文，过期清理先认证所有记录再开始删除，避免因先扫描到正常项而在后续发现篡改前删除。所有入口每次重新验证 Activation 和独立密钥，响应使用 `Cache-Control: no-store`。

删除和清理只移除 `<state>/raw-task-content/` 中的密文封套，不修改回执、效果证据、签名追溯包、原文 Grant/Revocation 或任务完成状态。测试以回执链长度和其他任务的 active 文件验证这一边界。跨任务读取按不可验证状态失败，不允许借 record_id 枚举其他任务。

验证通过：

- `go test ./...` 与 `go vet ./...`（`apps/agentshield`）。
- `go test -race ./internal/rawcontent ./internal/server -run 'Test(ListMetadata|PurgeExpired|RawTaskContentRecord)' -count=1`。
- Record、Records、Read Content、Delete 与 Purge 的固定 Go 样例通过 Python Draft 7 合同；合同测试共 196 项，Ruff 通过。
- `CGO_ENABLED=0` 下 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标编译通过。
- `gofmt -l` 与 `git diff --check` 无输出。

边界：本批尚未实现管理 UI 的明文二次动作、持续开启提示、自动定时清理、诊断包排除检查或原生平台采集。过期清理在完整认证后逐文件删除；底层文件系统删除失败仍可能形成可查询的部分清理结果，调用方收到错误后必须重新 search，不得假设全部成功。四目标编译不是 macOS/Windows 原生运行证据。本批未提交、推送或合并。
