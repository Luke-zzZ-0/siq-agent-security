# N01 状态写入口覆盖（独立审查修订）

状态：**组件 verified，N01 doing**。此前声称全部入口受保护和可重启迁移的清单已作废；原文在审查前快照中保留。当前结论及原始失败证据见 [审查报告](../ornith-n01-review-fixes-20260913-182231/report.md)。

| 入口 | 当前检查位置/顺序 | 回归证据 | 边界 |
| --- | --- | --- | --- |
| CLI 管理入口 | main 分派前 RequireStateCompatibility | TestCLIIncompatibleStateNoSideEffects | 先于命令参数依赖/业务副作用；不等于各 OS 实机服务操作 |
| serve | 解析实际 --state-dir 后，Open/密钥/后台任务前 | TestExplicitServeChecksSelectedDirectoryOnly；Linux 原生指定目录 | 环境目录不覆盖显式目录 |
| CodeBuddy hook | codeBuddyClient → Open 失败 → 结构化 deny | TestCLIIncompatibleHookEmitsBlockingDecision | 不用可能非阻断的单纯 exit 1 |
| state.Open | 检查根/祖先/既有核心目录/标记后才 MkdirAll | TestCompatibilityStrictMarkerAndZeroWrites、TestCompatibilityCoreDirectorySymlinkNoPartialOpen | 静态检查，不宣称抵抗任意同 UID TOCTOU |
| 根 Writer | mkdir/新锁/隔离旧锁前检查；取得锁后复验 | StrictMarkerAndZeroWrites、WriterRootIsNeverInferredFromBasename | 不从目录名猜根 |
| 维护 Writer | AcquireScopedWriter(stateDir, scope) 显式根；四类 scope | StrictMarkerAndZeroWrites、WriterRootIsNeverInferredFromBasename | 服务、适配器、client-release/snapshot 保留原有锁层次 |
| init | 验证 writer 所有权后预检；完成后不可变增加 marker | 初始化/既有 marker 保持测试；Linux init 重复与无标记场景 | 不转换旧业务格式，不自动备份 |
| 配置/token/admission/evidence/policy/version/audit/Grant commit/recover | 核心 Store 写前复验 | TestCompatibilityExistingStoreRejectsWrites（10 个边界） | 单独子存储直接调用仍需继续枚举 |
| Server 构造 / HTTP | 创建子存储前检查；HTTP 进入业务路由前检查 | TestIncompatibleStateRejectsHTTPAndMaintenance | 不兼容时 503，UI 恢复流程待完善 |
| 原文后台清理 | PurgeExpiredRawContent 开头复验 | TestIncompatibleStateRejectsHTTPAndMaintenance | 其他后台独立 writer 仍按后续总表核查 |
| client stage/snapshot | 创建私有存储前检查根，随后拿显式 scoped writer | clientrelease 全量业务回归 + scoped writer 拒绝测试 | 各发行系统的真实升级/回退另验收 |
| 自动迁移 | 已移除未完成引擎，显式旧格式拒绝，遗留 migration-plan 保留并拒绝自动恢复 | Explicit-old/无标记/migration-plan 拒绝与零写入测试 | 转换器、备份和恢复协议均未完成 |

数据合同：`packages/contracts/local-state-format.v1.schema.json`；Go 样例：`apps/agentshield/testdata/contracts/local-state-format.json`。JSON schema 可表示未来整数格式，运行时支持范围仅为 1；两者不是同一判定。重复键与多 JSON 值由原始字节解析器拒绝，schema 不能替代它。

缺失标记只接受明确的旧版结构/配置/身份和既有独立子存储，不能给未知非空目录自动发放新格式。独立 reader/writer 协议、实例绑定、旧程序实测、迁移崩溃恢复与三 OS 原生验收继续待办。不得以本表替代全 N01 验收。
