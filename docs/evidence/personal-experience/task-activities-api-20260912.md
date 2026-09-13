# M93：任务活动列表 API

日期：2026-09-12。规格 §3.12.2，合同 local-task-activities/v1。本地增量未提交或推送。

新增 GET /v1/task-activities，管理会话鉴权，任务与未归属双视图。limit=1..100、offset 和 snapshot 防止翻页混入新链状态；未知、重复和非法参数拒绝。Engine 锁内读完整快照，限额 10 万记录/64MiB，截断或内存链头不一致拒绝；复用已配置检查点和任务聚合验证。列表只输出绑定和回执计数/序号，不输出参数原文，不表示结果已核验。

## 验证

- Go HTTP 测试覆盖第一页/末页、不重复活动、未归属记录、后续页缺快照、非法参数、匿名 401、决策凭据 403、非 GET 405、新回执引起旧快照 409、空链成功与篡改链失败且不输出列表或篡改内容。真实 JSON 输出与共用 fixture 逐字节比较。
- `go vet ./... && go test ./... && go test -race ./internal/server -run TestTaskActivities -count=1` 通过，定向 race 1.138 秒。
- `uv run --locked pytest app/tests/test_schema_contracts.py -q` 166 项合同通过；新增合同校验必需字段、非法前缀有效标志、secret 扩展、绑定与未归属关系。Ruff 首次发现一处新增长行，修正后通过。
- linux/arm64、linux/amd64、darwin/arm64、windows/amd64 全包构建通过，格式与 git diff --check 通过。

## 后续

当前仅列表 API，任务详情、分页回执、作用域结果核验和可信 Skill 版本仍待接入；前端暂未使用此入口。链前缀验签不等于完整历史证明，结果核验也不来自列表。超大链返回不可用，不静默截取前段；后续可增量优化索引，但不能放松完整性语义。UX-011 保持进行中，无新增原生平台支持声明。
