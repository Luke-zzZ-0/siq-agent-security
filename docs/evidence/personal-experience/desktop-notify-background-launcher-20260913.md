# M130 证据：UX-008 后台启动器通知层（daemon 侧桌面通知，2026-09-13）

## 1. 范围与动机

UX-008（后台启动器）中"确认收件箱有新待办时主动提醒用户"的通知层。此前用户必须主动打开
本地控制台或轮询 `/v1/confirmations` 才能发现有被 hold 住的工具调用；本批在 daemon（serve）
侧加入一个可选的桌面通知调度器，把"有待确认操作"这一事实推到桌面。

不在本批范围（保持诚实登记）：
- **OS 实机投递证据**：本批交付的是 daemon 侧调度与投递层，全部测试以测试替身（stub notifier）
  和真实引擎组合完成；未在真实桌面环境（notify-send 实际弹出）验收，因此不声称 OS 实机投递。
- **原生恢复执行**：仍受 Hermes 30s 回调上限阻塞（M16 已验证记录），本批不涉及。
- **任务内授权**（task-scoped authorization）：后续批次。

## 2. 设计

### 2.1 opt-in 与平台诚实性

- 配置默认关闭：`desktop_notify`（bool）、`desktop_notify_command`（string），
  两者均 `omitempty`，旧 config.json 零变化（M128/M129 同原则）。
- 平台默认通知器仅在 linux 且 `exec.LookPath("notify-send")` 命中时启用
  （`notify.DefaultCommand`）；windows/darwin 一律报告不支持。
- **不支持平台 + 未配置 override → nil notifier + 显式日志**：
  `desktop notifications enabled but no notifier available on <goos> and none configured;
  confirmation inbox remains fully usable` —— 绝不伪造投递成功。收件箱功能不受影响。

### 2.2 隐私：count-only 通知体

桌面通知可被同机其他应用读取（与 ADR-032 浏览器通知同一隐私规则），因此通知体只含计数：

- Title：固定产品名常量。
- Body：`有 %d 项待确认操作，请在本地控制台处理`。
- **不含**：工具名、action_id、session/agent 标识、参数摘要、grant/receipt 标识。
  单测对 `action`、`tc-`、`sess`、`inst_`、`read_file`、`digest`、`token`、`grt-`、`adm-`、
  `receipt` 等泄漏面做了逐项否定断言。

### 2.3 投递语义（Dispatcher）

- 轮询 5s、合并窗口 15s（对齐 M15/M16 web 通知节奏）。
- 仅在 pending 数**相对上次已报告值增加**时投递；窗口内新增被抑制（不吞计数，
  抑制分支保持 lastCount 于已报告值，窗口过后下一 tick 仍能看到增量）。
- 计数归零后重置合并窗口：清空后再来新事项立刻提醒，不被陈旧窗口吞掉。
- 投递失败：记日志、**不盖 lastNotify 戳** → 下一窗口过后有界重试；永不影响决策路径
  或确认收件箱（调度器独立 goroutine，`defer cancel()` 退出）。
- `CommandNotifier`：argv 由 `strings.Fields` 切分、`exec.CommandContext` 直接执行
  （**不经 shell**），5s 超时，argv = 配置参数 + Title + Body。Title 为固定产品常量、
  Body 为计数句，无注入面。
- 配置校验在加载侧（`decodeConfig`）：空白命令拒绝（`state: invalid desktop_notify_command`）。
  `SaveConfig` 维持原状 —— 纯写盘（config.json 不承载安全决策），校验发生在读路径。

## 3. 变更清单

| 文件 | 变更 |
| --- | --- |
| `apps/agentshield/internal/state/state.go` | Config 新增 `desktop_notify` / `desktop_notify_command`（omitempty）；decodeConfig 校验命令非空白 |
| `apps/agentshield/internal/notify/notify.go` | 新包：Notification/Notifier/CommandNotifier/DefaultCommand/Dispatcher（Tick 合并语义） |
| `apps/agentshield/internal/notify/notify_test.go` | 新增 8 测试（见 §4） |
| `apps/agentshield/cmd/agentshield/desktop_notify.go` | desktopNotifier（配置优先→平台默认→nil）/pendingConfirmations（只数 Status=="pending"）/startDesktopNotify |
| `apps/agentshield/cmd/agentshield/main.go` | cmdServe 装配：startDesktopNotify + 产品前缀 stderr 日志 + defer cancel |
| `apps/agentshield/cmd/agentshield/desktop_notify_test.go` | 新增 6 测试（见 §4） |

接线只发生在 serve 路径（cmdServe），引擎热路径零改动；`pendingConfirmations` 复用
`Engine.Confirmations()` 只读投影（其只读性已有专门测试保护）。

## 4. 测试（14 项，全部通过；1 项在本机按设计 SKIP）

internal/notify（8）：
1. TestDispatcherNotifiesOnCountIncrease —— 计数上升即投递
2. TestDispatcherCoalescesBurstWithinWindow —— 15s 窗口内爆发合并，且不吞未报告增量
3. TestDispatcherReturnsToZeroThenReNotifies —— 归零重置窗口，清空后再提醒
4. TestDispatcherDeliveryFailureRetriesAfterWindow —— 投递失败记日志、窗口后有界重试
5. TestNotificationCarriesCountOnlyNoRequestMetadata —— count-only 泄漏面否定断言
6. TestCommandNotifierAppendsTitleAndBodyNoShell —— argv 拼接、无 shell
7. TestDefaultCommandOnlyLinuxWithBinary —— 仅 linux + notify-send 在 PATH
8. TestDispatcherRunStopsOnContextCancel —— Run 随 ctx 退出

cmd/agentshield（6）：
1. TestDesktopNotifierConfiguredCommandWins —— 配置 argv 覆盖平台默认，无 shell 切分
2. TestDesktopNotifierUnsupportedPlatformWithoutOverrideStaysSilent —— 不支持平台 → nil（本机 notify-send 存在，按设计 SKIP）
3. TestStartDesktopNotifyDisabledAndUnsupportedAreNoOps —— 默认关闭/无通知器均不启动，日志如实说明
4. TestPendingConfirmationsCountsOnlyPending —— 真实引擎（rulepack.Builtin + OpenChain + 真实
   grant：exec + credential 事实使 exec require_approval）hold 后计数 1，ResolveConfirmation 后归 0
5. TestDesktopNotifyConfigValidation —— 空白命令加载被拒；合法命令 SaveConfig/LoadConfig 往返
6. TestDispatcherPendingCountBindsDeliveryErrorToLog —— pending 闭包绑定引擎计数、失败入日志

## 5. 验证记录

| 检查 | 结果 |
| --- | --- |
| `go test ./...`（agentshield 全包） | 全部 ok，无 FAIL |
| `gofmt -l .` | 无输出 |
| `git diff --check` | 无输出 |
| `go vet ./...` | 无输出 |
| Python 合同测试（apps/control-api） | 197 passed（M130 未触碰合同 schema/样例，符合零变化预期） |
| `uv run ruff check app` | All checks passed |

四目标构建（CGO_ENABLED=0，`go build ./cmd/agentshield`，自 apps/agentshield）：

| 目标 | SHA-256 |
| --- | --- |
| linux/amd64 | 6bb8db504a07fc5574d5767f4858e950689e2931d222565703a42c7a8e1bbdb7 |
| linux/arm64 | aaf0df20e4707351dadf0c059ced2ab304ac1dc77d324c31c0d93d0760d7be4b |
| darwin/arm64 | c08eb1dd926c9221b0da790c69de6be9a6d4795fa8769b597e1e77cbf2e25ee2 |
| windows/amd64 | e4ec9fcbb03454fe50b9fe6a6f06c0183b75b6d747114cc4a6d3962422f430f1 |

## 6. 边界登记（诚实记录）

1. **OS 实机投递未验收**：调度/投递层已实现并有测试，但"桌面真的弹出通知"需真实桌面
   环境证据（notify-send 实际调用、windows/darwin 通知路径）。UX-008 的三系统实机验收
   项保持未完成状态。
2. windows/darwin 无平台默认通知器（Darwin osascript/UserNotifications、Windows Toast
   均未实现），仅可通过 `desktop_notify_command` 显式配置。
3. 通知内容恒为 count-only；若未来需要携带更多上下文，必须先过 ADR-032 同级隐私评审。
4. 引擎热路径零改动；调度器是 serve 侧只读观察者。
