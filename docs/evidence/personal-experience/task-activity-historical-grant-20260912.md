# M103 历史授权来源关联核心

日期 2026-09-12；UX-011；规格 §3.12.12。

新增 internal/intent/historical_grant.go 和测试。完整关联键来自已验签回执；读取既有不可变 Binding 与 Intent 并逐项匹配，返回历史 GrantReference 值副本。不会查询当前 Grant，不返回执行授权，缺失签名选择拒绝。暂不新增 wire 合同。

验证通过：

- 定向 TestHistoricalGrant，真实本地签名的意图/绑定/授权样例。
- 所有八个关联字段的错配、缺失摘要、篡改 Binding、缺少选择均拒绝。
- 安装会导致测试失败的 live Grant lookup，证明历史读取不依赖当前权限；真实签名撤销后 ResolveBinding 拒绝，而历史来源保持可读。
- go test ./...（日志 /tmp/siq-m103-go-test.log）、go vet ./...。
- go test -race ./internal/intent -run TestHistoricalGrant；新增撤销/缺失选择后再次定向 race 通过。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 go build ./...。
- git diff --check。

限制：仅为历史授权来源读取核心，未接前端/API，尚未从准入或安装清单解析 Skill 内容版本，更不能据此声称某次工具调用执行了该 Skill。无真实平台或原生系统新增验收。未提交推送。
