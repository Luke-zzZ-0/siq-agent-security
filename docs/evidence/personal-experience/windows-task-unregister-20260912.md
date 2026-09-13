# M88：Windows 任务注销

日期：2026-09-12。规格 §3.11.50，本地候选基于 274ed97；未提交或推送。

新增 `task-unregister --confirm-unregister`。持生命周期与主 Writer，核对签名源、定点存在性、完整系统配置和空闲状态；删除精确任务后读回缺席，本地配置、密钥和历史保留。已缺席可幂等确认；查询失败或删除失败不报成功。

删除脚本再核对当前用户、任务路径、最后已验过的 XML 快照及 ready/0 实例。XML 比较禁止 DTD/外部解析器，比较文档根 OuterXml；序列化差异也可能导致保守拒绝，需原生验收。使用 [Microsoft TaskFolder.DeleteTask](https://learn.microsoft.com/en-us/windows/win32/taskschd/taskfolder-deletetask) 的精确名称和 flags=0。系统 API 不提供条件删除事务，最后检查与删除间仍有同权限外部修改窗口，不声称可以抵御同权限恶意竞争者。

## 验证

- `go test ./cmd/agentshield -run TestWindowsTaskUnregister -count=1`：10 项模拟场景及确认参数负向通过。场景为空闲、已缺席、运行、排队、未知配置、存在性查询失败、删除失败、删除后仍存在、最终查询失败、初始缺席后重新出现；核对删除次数、删除前已验证快照、本地归属记录保留。
- `go vet ./... && go test ./... && go test -race ./cmd/agentshield` 通过，CLI race 7.362 秒。
- 四目标构建通过；`git diff --check` 与新增 Go 文件格式检查通过。

| 目标 | SHA-256 |
| --- | --- |
| linux/arm64 | c619fc148de05d80eb93040a90ad66e90310b11f3e3ad9bb951c4f913f03344f |
| linux/amd64 | d632204039dde7171029166a5efc0039231dc1ee31f1a5c3809d6a738de63d4f |
| darwin/arm64 | e6b3732d99d742e06987f748b9cbfe5124984c33af8c023cfc09bee3a5779aab |
| windows/amd64 | 9964ce381bfcb81eb6ac652fcbeb286b4ccfebf4324e607176876f704c1dad4b |

## 待验收

测试使用临时状态目录和模拟系统回调，未执行真实删除。环境没有 Windows/PowerShell 宿主，脚本解析、COM XML 一致性、任务状态与删除后的真实缺席均未原生验证，不计为原生支持。后续继续 Windows setup/teardown 和跨 OS 完整生命周期验收，UX-003 保持进行中。
