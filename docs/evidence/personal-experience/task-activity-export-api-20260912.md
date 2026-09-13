# M101 单活动签名下载 API

日期：2026-09-12。任务 UX-011；规格 §3.12.10。

实现：internal/export/task_document.go 与 internal/server/task_activity_export.go。新增 packages/contracts/local-task-activity-export.v1.schema.json，固定样例 testdata/contracts/local-task-activity-export.json。

GET /v1/task-activities/:id/export 要求管理身份与 snapshot。引擎读取完整受限快照后按绑定导出，最多 10000 行，签名后复验快照。attachment 文件名只使用已验证的十六进制活动 ID，响应 no-store。摘要包含整份源快照的链头/条数和单活动脱敏行，签名范围明确为 share_projection_only。

验证：

- Go 全量 `go test ./...`、`go vet ./...`；日志 /tmp/siq-m101-go-test.log。
- 定向 race：`go test -race ./internal/export ./internal/server -run 'TestActivityExport|TestTaskActivityExport'`。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全包 `go build ./...`。
- `uv run --locked pytest app/tests/test_schema_contracts.py -q`：170 项通过；Ruff 通过。
- 追加磁盘篡改用例后定向 TestTaskActivityExport 再次通过；固定样例不用生成开关也匹配。
- `git diff --check` 通过。

API 实测使用隔离临时状态、真实本地签名三条回执，只有序号 0、2 进入导出；敏感文本不透传。匿名/决策凭据、POST、缺少快照、分页、未知活动、错误视图、旧快照均拒绝。磁盘篡改返回 500 且无附件；错误公钥与修改行内容验签失败。Python 用固定测试可信公钥独立验证同一 Go 样例，修改快照/范围/行时签名失败。

限制：前端按钮尚未接入；导出只包含操作摘要，尚无 Skill 版本与效果材料。附带公钥不能自证身份，接收方须独立核对可信公钥。没有 macOS/Windows 原生或真实智能体平台验收新增证据；本批未提交、推送或合并。
