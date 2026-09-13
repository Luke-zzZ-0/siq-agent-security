# M92：任务结果核验的主体隔离

日期：2026-09-12。规格 §3.12.1，本地未提交增量。

新增 completion.EvaluateForSubject 内部核心。要求完整平台、会话、主体范围；证据先验签、拒绝重复 ID，经可信历史行动核对 action/decision_receipt/task，再按主体和意图版本筛选。筛选后的证据交现有 Evaluate 判定，不重复读取历史行动，不把其他分组的成功材料计入当前分组。无证据保持 incomplete，无要求仍由既有规则返回 unknown。

## 验证

- `go test ./internal/completion -count=1` 通过；新增 10 项范围场景使用真实签名的文件效果材料，覆盖匹配、跨平台、跨会话、跨主体、跨意图、摘要不同、行动引用错误、证据篡改、重复 ID 和缺失主体。只有匹配范围得到 verified，合法其他范围得到 incomplete，非法输入报错；每次历史行动只读一次。
- `go vet ./... && go test ./... && go test -race ./internal/completion` 通过，completion race 1.174 秒。
- linux/arm64、linux/amd64、darwin/arm64、windows/amd64 全包构建通过；格式与 git diff --check 通过。

## 尚未完成

未改变现有 /v1/tasks/<id>/completion API，也未暴露新 API 合同。后续任务详情必须使用已验证活动分组的完整身份调用此核心，并接可信 Skill 版本、证据引用和未知原因。前端、导出与真实平台验收仍待完成，UX-011 不标完成。本批不涉及远端提交/推送。
