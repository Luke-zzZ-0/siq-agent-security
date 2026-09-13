# M129–M130 独立阶段验收（2026-09-13）

> 后续状态：本文记录的四项缺陷已经修复并通过复验，见 [修复复验记录](stage-fixes-m129-m130-20260913.md)。以下保留原始验收失败事实，不将后续修复写成早期版本已通过。

结论：**M129、M130 暂不整体验收通过**。既有测试通过，但补充验收复现 2 项 P1 和 2 项 P2；修复后应按本文条件复验。本文是独立审查结果，不改写 GLM 原始开发记录。

范围：最新已登记完成的 M129（场景模板、多 Skill 调用边界）和 M130（后台桌面通知）。M124–M128 沿用前次阶段审查与修复记录。仍在开发的 UX-009 Git 导入不纳入本次功能验收。

方式：复制当时工作区至 `/tmp/siq-m129-m130-acceptance-20260913` 后运行测试。业务代码、用户配置和后台服务均未修改；新增验收用例仅写入隔离副本，仓库仅增加本报告与证据。结束前复核涉事源码与快照一致。HEAD 仍为 `274ed97`，工作区包含后续未提交成果，不能以 HEAD 单独代表被审源码。

## 1. [P1] 无执行／沙箱只读模板仍允许进程执行，部分情况下还移除了审批要求

位置：`internal/grant/scenario.go:34`、`internal/grant/grant.go:239`、`internal/receipt/engine.go:801`（均相对 apps/agentshield）。

模板删除 process/resource 声明，却保留所有 tool 声明。原始准入同时包含 `tool.invoke terminal/exec` 与 `process.exec` 时，执行工具仍进入允许列表；运行时按该列表裁决，未由 Scenario 额外拒绝进程执行效果。OpenClaw 的 process 声明原本生成 exec 审批要求，删除后仅剩显式工具 allow，反而从 hold 变成 allow。

隔离测试使用真实 grant.Build → Approve → MarkDeployed → receipt.Engine.Decide，block 模式，请求 `printf review-marker`。这是裁决链测试，没有实际运行该 shell 工具。

| 平台 | 不选模板 | no-exec | sandboxed |
| --- | --- | --- | --- |
| Hermes terminal | allow | **allow，预期 deny** | **allow，预期 deny** |
| OpenClaw exec | hold | **allow，预期 deny** | **allow，预期 deny** |

建议修复：限制须落到规范化操作效果及所有运行时授权投影，不能仅删除声明域或只处理一个工具别名；审批约束不得因选择限制模板消失。新增 Hermes/OpenClaw 实际裁决测试覆盖 exec/terminal、直接工具声明、审批保持及正常只读操作。`sandboxed` 不能作为已建立 OS 沙箱的支持宣称。

## 2. [P1] 同一智能体更换场景被接口静默忽略

位置：`internal/server/server.go:803`、`internal/grant/grant.go:190`。

Grant ID 不包含场景，创建路由发现同 ID 的 live Grant 就直接返回旧对象，未比较请求中的 scenario_id。复现：同一 admission/platform/subject 先创建基线授权，再 POST `scenario_id=no-exec`，响应为 HTTP 200、`reused=true`、`scenario=null`，旧授权仍是 pending_approval。IsLiveStatus 还包含 approved/deployed/effective，因此路径不限于待审批状态。

这不是幂等的相同请求：用户选择了新的权限限制，接口却把旧授权作为成功结果返回。

建议修复：对比请求的完整授权意图；不一致时明确返回 409 并引导创建新草稿，或使用已有审核/版本机制生成独立待确认对象。不得静默复用旧权限，也不得直接覆盖有效授权。复验覆盖相同场景重复请求、更换场景、基线与限制模板互换、已有批准/部署授权。

## 3. [P2] 通知失败没有遵守承诺的 15 秒重试间隔

位置：`internal/notify/notify.go:155`、`:162`。

失败时不更新任何尝试时间，而限流只比较上次成功时间。首次失败或上次成功已过期后，每次轮询都会重新调用通知器。补充测试在 t=0、5、10 秒调用 Tick，得到 **3 次失败投递**，预期 15 秒窗口内仅 1 次。现有测试仅在 t+16 秒检查能否重试，没有测试窗口内不得重试。

建议修复：分别记录尝试时间和成功报告状态，或维护明确 nextRetryAt；保持“失败不吞待办”的同时限制子进程启动与日志频率。增加持续失败、窗口内抑制、窗口后恢复的用例。

## 4. [P2] 通知子进程原始输出进入 daemon 日志

位置：`internal/notify/notify.go:77`、`cmd/agentshield/desktop_notify.go:62`。

CommandNotifier 用 CombinedOutput 收集子进程输出，再把完整 stdout/stderr 拼入 error；启动器把 error 原样写日志。使用仅在隔离测试运行的子进程打印合成标记 `review-secret-marker` 并失败，标记出现在返回错误中。证明当前路径没有类别化、脱敏或输出长度限制；未使用或泄露真实凭据。

桌面通知正文只含计数这一项可以接受，但它不能证明日志也满足相同边界。自定义通知器可能输出本机路径、参数或环境诊断信息，原样持久化不符合本模块“日志只记类别”约定。

建议修复：仅返回稳定错误类别、退出码/超时状态，丢弃或严格限制辅助输出，避免无界 CombinedOutput；增加子进程输出不进入日志及超大输出有界处理测试。

## 已核验的成果与限制

- 原有 grant、notify、receipt、state 四包测试独立重跑通过。
- 场景 HTTP 接口和通知 CLI 接线定向测试通过；本机有 notify-send，**2 个测试发生 SKIP**，原报告“1 项跳过”应更正。跳过不作为通过。
- 原有场景、Skill 归属、通知相关定向 race 测试四包通过；相关包 go vet 通过。
- 新增验收负向测试复现上述四项问题：场景工具执行有 4 个失败子用例，API 选择场景、通知重试及日志各有失败用例。
- Skill 元数据精确匹配保持 unknown 的前次修复仍在；不能把使用 mocked verified lookup 的单测当成真实 Skill 执行来源已验证。
- 本次未做真实桌面通知投递、Windows/macOS 实机、真实用户审批恢复或完整九组合验收；M130 报告已注明其中多数缺口。
- M129/M130 的规格尚未完整回写主开发规格，修复时应同步，特别是限制模板的执行语义、场景切换和失败重试行为。

## 证据与复验入口

[核验清单](review-m129-m130-20260913/verification.json)、[原有测试输出](review-m129-m130-20260913/baseline.log)、[接口/接线输出](review-m129-m130-20260913/integration.log)、[race 输出](review-m129-m130-20260913/race.log)、[新增负向输出](review-m129-m130-20260913/negative.log)、[场景切换负向输出](review-m129-m130-20260913/negative-server.log)。

新增用例归档为 [reviewer-negative-tests.patch](review-m129-m130-20260913/reviewer-negative-tests.patch)。在包含本阶段代码的**隔离副本**中应用该补丁后，于 apps/agentshield 运行：

```bash
go test -count=1 ./internal/receipt ./internal/notify ./internal/server -run '^TestReviewer' -v
```

当前预期失败；修复后应全部通过，并重跑相关既有测试及原生场景。先修 P1，随后修通知可靠性/日志边界，再补实机证据。尚未提交、推送或合并本轮审查材料。
