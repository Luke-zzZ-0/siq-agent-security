# M97：活动范围结果核验 API

日期：2026-09-12。规格 §3.12.6，合同 local-task-activity-completion/v1。本地增量未提交。

管理鉴权 GET /v1/task-activities/:id/completion，以活动快照和完整绑定选择签名意图。任务/主体/平台/意图摘要必须相符，效果证据由历史行动读回及 EvaluateForSubject 核验；返回前再确认链快照未改变。未归属或缺少意图返回空结果与明确原因，不伪造完成状态。结果带本次 evaluated_at，不代表未来状态不变。

## 验证

- HTTP 覆盖无效果要求 unknown/not_required、缺少意图、未归属、拒绝决策凭据/POST 完成声明/分页参数、旧快照 409；Go 固定输出与 Python 共用 fixture，仅规范化 evaluated_at，运行时另验证合法时间格式。
- 已有真实文件观测 HTTP 场景增加活动范围入口：warn 合法写入材料得到 verified，block 下未授权已发生效果得到 conflicting。这里是临时目录内的实际文件变化与签名观测，不是智能体真实宿主执行验收。
- `go vet ./... && go test ./... && go test -race ./internal/server -run 'TestActivityCompletion|TestFileObservationHTTPReadsRealState' -count=1` 通过，定向 race 1.488 秒。
- Python 168 项合同与 Ruff 通过，包含 result/null 与 reason_code 关系、非法 completed 状态拒绝。
- 四目标全包构建、gofmt 和 git diff --check 通过。

## 后续

前端结果展示、证据详情、可信 Skill 版本与脱敏导出仍待接入，UX-011 不标完成。不改变旧 task completion API，不扩张原生支持声明，未提交或推送。
