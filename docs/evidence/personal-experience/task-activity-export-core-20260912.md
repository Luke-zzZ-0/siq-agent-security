# M100 单活动脱敏导出核心

日期：2026-09-12。规格：§3.12.9；任务：UX-011。

实现位于 internal/export/task_activity.go。复用 receipt.ProjectTaskActivities 对完整输入验签，精确匹配七字段绑定；不接受任意回执索引。未知绑定或预算不足返回错误及空结果。内部投影不改变已有导出合同。

测试用真实本地签名的三条交错会话回执，确认只选择序号 0、2；原因、参数摘录、工具/回执标识中的测试敏感文本不出现于投影。非允许裁决转 unknown，非法时间丢弃；有效时间规范为 UTC。无检查点保持历史 unknown；组外回执篡改同样拒绝。覆盖预算 0、恰好两条、一条不足及超过上限、未知会话。

验证通过：

- `go test ./internal/export`
- `go vet ./...`
- `go test ./...`（日志 `/tmp/siq-m100-go-test.log`）
- `go test -race ./internal/export`
- `GOOS/GOARCH` 为 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `go build ./...`
- `git diff --check`

限制：这是内部投影，不是可独立验签的导出包，不证明磁盘快照完整性或任务效果成功。接入方须使用受限引擎快照；下载合同、派生签名、API、UI 和实际使用验收待继续。构建不作为 macOS/Windows 原生证据。本批仅本地落盘。
