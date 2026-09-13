# N01 读写入口与例外清单

本表对应本批源码及 [验收报告](report.md)，不是“只在 serve 增加判断”的声明。底层封装是防漏补充，上层授权、签名、根目录识别和已有锁协议仍保留。

| 类别 | 入口/基础函数 | 保护 | 测试 |
| --- | --- | --- | --- |
| 状态根与初始化 | state.Open / Initialize / AcquireWriter / AcquireScopedWriter | 未识别目录拒绝；格式与根/实例检查在副作用前；旧历史不因缺配置自动变新状态 | Compatibility*、InitializeHistoricalStateDoesNotSkipMigration、BoundInitialization* |
| 本地 CLI | main 分派；serve 的实际 --state-dir | 格式前置；只读 state-status 单独诊断，state-migrate 仅固定迁移路径 | CLIIncompatibleStateNoSideEffects、StateProtocolCLIContractFixtures |
| HTTP/后台 | Server.New / Handler / PurgeExpiredRawContent | 构造和请求进入业务前复验；失败 503；rawcontent 本身文件访问也受底层约束 | IncompatibleStateRejectsHTTPAndMaintenance、全 server 回归 |
| 核心状态 | Token / SaveConfig / PutAdmission / PutEvidence / PutVersioned(CAS) / Grant commit/recover / 服务元数据 | Store 前置 + statefs 文件访问；serviceWriter 原所有权校验保留 | CompatibilityExistingStoreRejectsWrites、全 state 回归 |
| 密钥 | signing.Load / LoadExisting | 生成/读取文件经 statefs；纯 FromSeed 不访问状态 | IndependentWritersAndReadersRejectChangedFormat |
| 回执/检查点 | receipt.OpenChain / Chain.Append / checkpoint store | 读/创建/追加/HEAD 替换前经 statefs | IndependentWritersAndReadersRejectChangedFormat、全 receipt 回归 |
| 待处理记录 | pending.Append / Promote | 创建/追加/迁移待处理文件经 statefs；钩子仍结构化拒绝 | IndependentWritersAndReadersRejectChangedFormat、CLIIncompatibleHookEmitsBlockingDecision |
| 独立业务存储 | intent / runtimeidentity / effectevidence / rawcontent / notify 相关文件操作 | Open/ReadFile/CreateTemp/Link/Remove 等经 statefs，已有业务锁/验签不变 | statefs 负向 + 各包全量与 race |
| Skill 生命周期 | skillimport / skillinstall / adapterinstall / runtimecheck | 状态及私有工件读写经 statefs；外部平台副作用前保留 authority/state 预检 | 各模块全量与 race；模板不包含执行来源内容 |
| 制品与服务 | clientrelease Stage / Snapshot / Restore；service-upgrade/rollback 和 client-install/check | 新清单签名覆盖范围；匹配实际根状态后才 staging/恢复/停服；执行前 recheck 与原健康读回 | StateBoundReleaseBeforeAnyStaging、原有 service switch/rollback 回归、四项原生服务测试 |
| 读写原语 | statefs.WriteFile/OpenFile/Create/CreateTemp/Mkdir/MkdirAll/MkdirTemp/Remove/RemoveAll/Rename/Link/Symlink/Chmod；Open/ReadFile/ReadDir | 写操作检查 writer，读操作检查 reader；所有标记祖先都检查，嵌套旧标记不能遮蔽未来外层 | IndependentWritersAndReadersRejectChangedFormat、NestedMarkerCannotShadowOuterState |
| 迁移 | MigrateState；固定原路径元数据转换 | 主/四类维护锁；有界递归清单、私有备份、计划绑定与检查点；漂移拒绝 | FullBackupAndEveryCheckpointRecovery、RejectsDriftAndUntrustedPlan、ExclusiveAndUnsupportedSource、RecoversMissingMarkerOnlyAfterPreparedBackup、原生双二进制 |

## 明确例外

- `stateformat` 必须用原始 os 读取 marker/实例/活动计划，否则自身调用会递归。它只读取与验证，不授予业务权限。
- `state/writer.go` 用原始 os 创建经预检的独占锁、隔离已确认失效的锁，以及按 PID/owner 释放自己的锁；迁移中不能因屏障存在而无法释放自己的锁。
- `state/migration.go` 在持锁与计划校验后使用原始 os 写私有 journal/backup/检查点及精确兼容标记，最后清理匹配的活动屏障；没有调用方可传的任意目标路径。
- 测试 fixture、只在内存工作的操作、状态目录以外的文件仍由原模块约束。已经打开的文件描述符、任意旧程序或恶意同 UID 并发不因此成为 OS 隔离范围。

已替换的业务文件列表见 [statefs 接入清单](statefs-files.json)。审查搜索现有生产文件中的原始 os 写调用，只保留以上例外；这不替代未来新增入口的代码审查。
