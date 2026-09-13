# M111 完整追溯包下载 API

日期：2026-09-13；任务：UX-011/013；规格：§3.12.20。

新增管理 `GET /v1/task-activities/{activity_id}/trace-export?snapshot=...`。接口只接受已归属任务和明确源快照；从完整已验签回执链生成活动摘要，按每条回执主体及历史 Grant 解析 Skill 来源，复用活动完成结论的主体隔离核验，并只从本机签名效果存储读取候选记录。调用方不能提交任务 ID、来源、完成结论或效果证据。

服务端使用同一时间点构建 `local-task-trace-export/v1`，签名后重新读取回执快照并比较效果记录 ID/签名集合。任一集合变化返回 409，且不设置附件下载头。历史来源缺失或无效保留 `unavailable` 并令包为 incomplete；每次最多解析 8 个不同历史来源键，超限返回 413，防止完整下载造成无界磁盘读取。

验证通过：

- 新增接口测试覆盖独立签名验签、附件/no-store 头、来源关联、unknown 完成结论和空效果集合。
- 响应正文不含测试中的 Skill 名称/版本、回执、工具、任务、Grant 或 Admission 原始标识；删除历史准入后重新下载得到明确 unavailable/incomplete。
- 管理鉴权、决策令牌越权、方法、缺失/非法快照、分页参数、未归属视图、未知活动、过期快照及来源读取预算负向通过。
- `go test ./...`，日志 `/tmp/siq-m111-go-test.log`；`go vet ./...`；`go test -race ./internal/server -run 'TaskActivityTrace|ActivityCompletionUnknown' -count=1`。
- Control API 的 174 项 Draft 7 合同测试与 Ruff 通过。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 go build -trimpath ./cmd/agentshield` 通过。
- `git diff --check` 通过。

限制：当前只提供服务端下载能力，前端尚未显示“完整追溯包”入口。接口测试使用明确的本地签名 fixture，没有形成 OpenClaw、Hermes 或 WorkBuddy 的真实任务旅程证据；交叉编译不作为 macOS/Windows 原生验收。本批未提交、推送或合并。
