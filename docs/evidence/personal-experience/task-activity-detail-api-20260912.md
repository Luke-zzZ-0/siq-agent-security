# M95：任务活动详情回执 API

日期：2026-09-12。规格 §3.12.4，合同 local-task-activity-detail/v1。本地未提交增量。

管理鉴权 GET /v1/task-activities/<activity_id>，沿用列表快照与分页。仅返回选定任务分组的回执摘要；未归属活动按单条读取。保留工具、裁决、原因和授权引用，显式省略参数及摘录，未加入效果核验状态。

## 验证

- Go HTTP 测试构建四条真实签名回执（两个会话交错、一条未归属），验证分页取 seq 0、2 而非夹杂的 seq 1。核对总数、末页、未知活动、错误视图、旧快照、缺少后续页快照、匿名/决策凭据拒绝、未归属 seq 3、参数摘录不出现。
- 响应与共用 Go fixture 逐字节比较；Python 检查必需字段、非法 prefix_valid，以及 params/params_excerpt/token 扩展拒绝。
- `go vet ./... && go test ./... && go test -race ./internal/server -run 'TestTaskActivit' -count=1` 通过，定向 race 1.181 秒。
- Python 167 项合同测试通过；Ruff 发现新增 fixture 路径行过长，拆行后通过。
- 四目标全包构建及 gofmt/git diff --check 通过。

## 后续

前端尚未链接详情，作用域 completion 与可信 Skill 版本也未接入详情。仅有本机测试，不扩大原生平台支持声明；UX-011 保持进行中，未提交或推送。
