# M91：任务活动聚合核心

日期：2026-09-12。规格 §3.12，本地未提交增量。

在 receipt 内复用 VerifyDetailed，保留前缀、历史完整性和新鲜度的独立报告。前缀校验失败返回报告与错误且不生成可信分组。按链、平台、会话、主体、任务、意图和意图摘要分组，只有完整 bound 记录进入任务；缺失关联保留未归属索引。已撤销权限的历史绑定仍可归属，不说明权限有效。只读内存投影，无新外部 JSON 合同或持久化。

## 验证

- `go test ./internal/receipt -run TestTaskActivityBoundariesAndIntegrity -count=1` 通过。真实签名链包含 11 条交错记录，验证 7 个独立任务分组、3 条未归属记录、撤销记录回归原历史任务。覆盖跨平台、会话、主体、任务、意图和摘要隔离；篡改 task_id 后拒绝分组，无检查点不声称完整历史已核验。
- `go vet ./... && go test ./... && go test -race ./internal/receipt` 通过，receipt race 24.604 秒。
- linux/arm64、linux/amd64、darwin/arm64、windows/amd64 全包 `go build ./...` 通过。
- 新增文件 gofmt 与 git diff --check 通过。

## 尚未完成

当前为内部聚合核心，未接 API 或任务详情。结果证据需后续关联 completion/effect 的真实证据，不能用 allow 或工具调用成功替代；可信 Skill 版本、筛选、脱敏导出、真实平台样例也仍待完成。本批不扩大任何平台原生支持声明，UX-011 保持进行中。
