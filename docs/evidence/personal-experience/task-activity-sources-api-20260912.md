# M106 活动历史 Skill 来源查询 API

日期 2026-09-12；UX-011；规格 §3.12.15。

新增 internal/server/task_activity_sources.go 与 local-task-activity-sources/v1 合同。GET /v1/task-activities/:id/sources 使用管理鉴权、必须提供 snapshot，复用活动分页与完整链快照。M103/M104 核心通过 M105 受限读取核对历史准入，返回最小来源元数据。相同历史键缓存，每次最多 8 个不同键，超限返回 413；末尾复验快照。

验证通过：

- TestTaskActivitySources 在真实隔离状态签发意图/授权/绑定/准入，追加交错会话回执；正确输出版本 1.2.3 和内容摘要，第二页仅序号 2，其他会话 unavailable。
- 签名撤销后来源保留；准入篡改、缺失时 source=null，不透传材料或底层错误；未归属为 unattributed。
- 匿名 401、决策凭据 403、POST 405、缺少快照 400、错误/过期快照 409、不存在活动 404。
- 8 个不同键成功，9 个拒绝 413，追加回执后旧快照拒绝。
- go test ./...：/tmp/siq-m106-go-test.log；go vet ./...；定向 server race 通过。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全包 go build ./...。
- 固定 Go 样例 local-task-activity-sources.json 运行时比对，171 项 Python 合同测试与 Ruff 通过，含来源状态/空值约束负向。
- git diff --check。

限制：当前接口证明签名历史授权引用的分析来源，不证明某次工具调用实际执行 Skill，也不证明当前安装/权限有效。前端来源面板待接。签名样例来自测试构造，不计为真实智能体或 macOS/Windows 原生验收。本批未提交推送。
