# M108 全快照活动检索 API

日期 2026-09-12；UX-011；规格 §3.12.17。

新增 /v1/task-activities/search 管理查询及 local-task-activity-search/v1 合同。复用完整受限回执快照和任务投影，平台/主体/会话/任务精确条件与标识关键词采用 AND。关键词为大小写归一的字面子串，不执行正则、不搜索参数。先过滤再分页，保留原 activity_id 与源 snapshot，回显完整 filters。

验证通过：

- TestTaskActivitySearch：真实签名交错样例的中文关键词第一页 seq=1、第二页 seq=3；源 snapshot 与未筛选列表相同，可用原详情读取。
- 精确条件/组合无匹配、英文大小写、正则字符按字面、未归属筛选保持 binding=null。
- 参数 256 Unicode 字符通过，257 拒绝；重复参数、非法 UTF-8/控制符、未知参数及缺少后续页快照拒绝。
- 匿名 401、决策凭据 403、POST 405、未知/追加后的旧快照 409。
- 篡改磁盘链时即使关键词不匹配仍返回 500，不伪装为空结果。
- go test ./...：/tmp/siq-m108-go-test.log；go vet ./...；定向 server race 通过。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全包 go build ./...。
- 固定 Go 查询样例运行时一致性；172 项 Python 合同测试与 Ruff 通过。
- git diff --check。

前端筛选表单/查询恢复尚未接入。本批搜索仅覆盖任务、主体、平台、会话标识，不包含 Skill 名称/版本或效果材料全文。未提升原生平台验收声明，未提交推送。
