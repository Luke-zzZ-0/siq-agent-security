# 正式 Windows Go 运行：最终结果与残余问题

状态：**正式运行已结束，退出 1，未通过全量 Go 验证**。命令为 `go test -json -p 2 -count=1 ./...`，开始于 2026-09-14 00:52:54.359436 UTC，结束于 01:27:34.139282 UTC。本报告在结束通知后只对最终日志再读取一次；最终长度 2,366,867 字节、9,859 条有效 JSON events，SHA256 为 `c2029645386d6f28e9fef82633c377ca5528893a20215dbad40fd9738c745a14`，均已与批次记录核对。

受测实现候选：`ebc472f2e46aa7de837afe9d6a0ed422eef51cd0`，`source_dirty=false`。简洁的计数、失败包及顶层失败测试名见同批 `go-final-summary.json`。历史基线与中途快照只用于定位，未把其失败数或超时替代最终结果。原始日志包含私有测试路径，仅本机保留；本文只列测试名、公开源码位置和脱敏诊断。

本次没有修改代码、样本、skip 条件或测试分母，没有启动额外 Go 测试，没有新建 Issue。以下建议不代表已修复，也不把前置条件失败改成通过。

## 1. 结论边界

最终失败包括：跨 OS 测试路径假设、Windows 符号链接构造权限、POSIX mode 断言、未安装 wrapper 的存在性断言、未转义 JSON、打开文件后的替换构造；Windows 文件资源、Connector/制品可执行入口、崩溃后 Writer 接管和迁移恢复的实际能力缺口；以及尚未完全定位的合同样例差异、Git fixture、性能/时限和 shell 诊断。

`file_observation_unavailable` 与 `runtime_check_intent_failed` 的多项真实失败发生在前置材料阶段，阻止后续完成证明、审批/清理和篡改断言；不能仅按测试名称声称“假成功被接受”或“损坏记录通过”。最终仅 **server、skillinstall 两包**出现明确 `panic: test timed out after 10m0s`；state 包正常结束并报告断言失败，不能按耗时推定超时。

| 统计层级 | pass | fail | skip | 终态合计 |
| --- | ---: | ---: | ---: | ---: |
| 包 | 23 | 16 | 6 | 45 |
| 顶层测试（名称不含 `/`） | 934 | 61 | 48 | 1,043 |
| 子测试（名称含 `/`） | 1,110 | 80 | 19 | 1,209 |
| 全部测试层级事件 | 2,044 | 141 | 67 | 2,252 |

统计按 `(Package, Test)` 的终态事件去重；顶层测试和子测试是不同层级，不能相加作为顶层测试分母。另有 **2 个已经 run、没有终态的顶层测试**，均被包级超时中断，未计为 pass/fail/skip。包级 skip 不自动等于测试用例跳过。

本批 Windows/StateStatus 定向 Go 验证另有正式 `candidate-windows-go-tests.json`：同一候选、干净源码、命令 `go test -p 1 -count=1 -timeout=10m ./cmd/agentshield -run ^Test(Windows|StateStatusReportsAncestorBarrier)`，退出 0。该专项与全量退出 1 分别保留，专项不能替代全量。

独立边界 race 验证也已完成并退出 0：`internal/stateformat` 1.495 秒，`internal/statefs` 3.170 秒；编译与执行整体为 2026-09-14 01:27:35.631035–01:33:43.793559 UTC。其两条 `ok` 输出日志 SHA256 为 `858cdc852d4022b2c85a665d13a5fdd3d9e970ddea00a834e8b214a6cc9a80fa`。该结果只覆盖这两个边界包，不是全量 race 通过，也不改变上方全量普通 Go 失败结果。批次最终进程观察见 `final-process-observation.json`，私有测试根下原生进程数为 0。

## 2. POSIX/其他 OS 测试假设与 Windows 构造前提

| 已发生的正式失败 | 最小源码证据 | 分类与不能推导的结论 | 后续主修与 Windows 复验 |
| --- | --- | --- | --- |
| `cmd/agentshield` 的 `TestLoadRegisteredLaunchAgent`、`TestStartRegisteredLaunchAgent`、`TestStopRegisteredLaunchAgent`、`TestUnregisterLaunchAgent`、`TestTeardownLaunchAgentRecoveryAndRetention` 及其失败子例；诊断为 `launch-agent: canonical absolute UTF-8 paths required` | 例如 `launch_agent_load_test.go:18,34`、`teardown_launch_agent_test.go:17,33`：状态目录来自 Windows `t.TempDir()`，再与 POSIX 可执行路径一起传给 `renderLaunchAgent` | macOS LaunchAgent 测试混用宿主本地路径，渲染前置检查已拒绝，尚未进入模拟系统管理器的核心断言。不能记作 Windows Task Scheduler 失败，也不能证明 macOS 原生行为失败 | GLM/测试负责人明确纯渲染 fixture 与真实 OS 生命周期测试的边界，构造合法且身份一致的目标 OS 输入；Windows 负责人复跑受影响测试，Luke 保留 macOS 原生验证。不能为过测放宽 canonical path 检查 |
| `TestPrepareRollbackMissingBinary`；诊断为 `service-unit: absolute paths required; executable path cannot contain dollar signs, double quotes or backslashes` | `service_restore_test.go:22–25` 将 Windows `filepath.Join(t.TempDir(), "original")` 交给 systemd `renderUserUnit` | Linux 服务单元 fixture 的路径前提失败，尚未到达缺失二进制 rollback 断言。没有证明 Windows 升级/回退支持或缺失二进制被放行 | GLM/服务测试负责人区分 systemd 场景与 Windows 对应实现；Windows 另测原生制品恢复，保留缺失目标/签名/摘要/状态不兼容时副作用前拒绝 |
| `TestServeStateDirectorySelection`、`TestUnregisterExactLinkAndRetry/{false,true}`、`TestUnregisterRejectsUnknownAndFailedReload`、`effectevidence/TestStoreRejectsUnsafeStateAndInput`、`inventory/TestMCPSymlinkIsSkipped`；均报 `A required privilege is not held by the client` | 对应测试分别在 `serve_state_directory_test.go:27`、`service_unregister_test.go:24,29,86`、`store_test.go:93`、`inventory_test.go:464` 调用 `os.Symlink`，失败后立即 `Fatal` | 攻击/系统链接 fixture 没有成功创建，目标防护断言尚未执行。不能记成 symlink 防护通过，也没有足够证据判断链接已越界或 ACL 泄漏 | GLM/Windows 文件安全负责人定义受支持的 Windows fixture 和测试能力记录；保留真实 symlink/junction/reparse 的独立验证。不要关闭防护、全局要求管理员，或通过新增 skip 抹掉缺口 |
| `adapterinstall/TestUninstallOfOneInstanceRestoresOnlyThatInstance`；`sibling wrapper removed by other instance's uninstall` | `backup_restore_test.go:136–138` 无条件要求 wrapper 存在；`plan.go:436–437` 明确只有非 Windows 才安装 Hermes shell wrapper。前面的 sibling config/plugin 断言已执行到此处 | 测试期待了 Windows 从未安装的 wrapper；消息本身不能证明另一个实例的文件被误删。Windows 的接入入口能力仍需独立界定 | GLM/安装器测试负责人校准 OS 预期对象；Windows 复验已有 config/plugin、备份和登记对象的归属保留，并单列 Windows 受支持安装入口，不能简单删除所有隔离断言 |
| `adapters/TestPolicyExecBlocksQuarantineWarnsConditionsAllowsClean`；`response must carry admission id and verdict` | `adapters_test.go:35` 把真实 `filepath.Abs` 直接拼进 JSON 字符串；Windows 反斜杠未 JSON 转义。`adapters.go:63–66` 在 JSON 解码错误时返回 fail-closed block，不产生 admission | fixture 序列化错误，测试可能在准入前被拒。当前失败没有证明恶意 Skill 被允许，也未完成正常/条件/恶意三组准入验证 | GLM/适配器测试负责人使用类型化 JSON 编码保留原路径，随后 Windows 复跑全部三组行为、输出关联和 malformed 负例；不改决策逻辑使 malformed 产生准入 |
| `admission/TestAddFileDetectsReplacedInode`；`The process cannot access the file because it is being used by another process` | `reading_test.go:205–215` 持有 `os.Open` 句柄后调用 `os.Remove`；Windows 在删除步骤拒绝。`addFile` 的替换身份检查在后续代码才调用 | POSIX inode 替换构造未成立，没有到达受测防护。不能记成 TOCTOU 防护通过或被绕过 | GLM/Windows 文件安全负责人设计可证实前后文件身份不同的 Windows 变换 fixture；Windows 核对实际文件共享语义、句柄身份、无越界副作用，再保留拒绝断言 |
| `pending/TestAppendWritesUnsignedJSONL`；`pending log must be 0600, got 666` | `pending_test.go:49–50` 直接断言 `Mode().Perm()==0600` | POSIX mode 不是 NTFS ACL 测量。已发生测试失败，但不能由 0666 推导其他用户能读取，也不能反向宣称 ACL 已安全 | GLM/状态与 Windows 权限负责人明确平台等价保护目标；Windows 记录真实 DACL、继承和允许/拒绝访问证据，同时保留 JSONL 内容/未签名属性断言 |
| `runtimeidentity/TestIdentityIssuanceRestartRevocationAndReplacement` 与 `TestIdentityCorruptAndAliasedRecordsFailClosed/permissions` | `store_test.go:166` 检查 POSIX 私有权限；permissions 子例只 `os.Chmod(path,0644)`，随后要求认证拒绝。`files.go` 的权限位检查明确排除 Windows | 仍是正式失败，但 chmod 没有证明 Windows DACL 或签名内容遭篡改；`corrupt authority accepted` 文本不能单独证明 Authority 绕过 | GLM/身份与 Windows 权限负责人定义真实 ACL 负向；Windows 记录攻击确实改变权限、认证拒绝及原文件保留 |
| `skillimport/TestImportTamperAndAnalysisBinding/executable`、`state/TestUnmarkedMigrationPreservesPrivateReadonlyTree`、`skillmanifest/TestDownloadVerifiedArtifactHappyAndNegatives` | 分别只 chmod 0700 后期望身份变化、断言目录 mode 恰为 0500、断言下载文件有 POSIX execute 位 | 测试中的 POSIX mode 假设；不证明可执行载荷越权、源目录真实 DACL 变化或制品签名绕过。下载成功后的测试断言与下节 staging 产品拒绝是两件事 | GLM/对应模块明确 Windows 身份/只读/执行语义；Windows 核验实际 ACL、PE/制品身份、摘要与副作用，保留跨平台对等测试 |
| 后续实际出现的 symlink 构造失败：`runtimeidentity/.../{ancestor_link,record_link}`、`skillimport/TestImportLocalSymlinksAndWrongSigningKey/{false,true}`、`skillinstall/TestInspectionSeparatesHistoricalRecordAndCurrentContents/type_changed`、`skillmanifest/TestPythonHashSkillDirRejectsEscape` | 全部在 `os.Symlink` 后报相同 privilege 错误 | 均未完成攻击构造；不能把测试名中的逃逸/签名/类型变更当成实际到达的防护断言 | 与上方 Windows 链接 fixture 统一安排，不新增 skip 或要求全局提权 |
| `skillinstall/TestInstallationFailureRollbackAndUnknownOwnership/case-alias`；实际为 `recovery_required` / `skill_install_conflict`，测试要求 `rolled_back` | `operation_test.go` 在计划目录大写变体下加入 `user.txt`；分支只把 foreign-file/modified-file/missing-owner 视为须人工恢复，未把 case-alias 纳入 | Windows 大小写路径别名会把“另一个目录”变成目标内用户内容。当前证据是拒绝自动回滚，不能声称用户文件被删除或安装被放行；是否应调整用例/恢复设计需先记录目录身份与内容 | GLM/安装器负责人明确大小写别名和未知用户内容归属；Windows 证明前后用户字节保留、拒绝范围及正确恢复路径，不能强制删除目标让 rollback 成功 |

## 3. 已发生的真实产品能力缺口

### 3.1 Windows 文件资源与文件观察：沿 Issue #39 协调

正式运行已出现：

- `completion/TestCompletionRequiresActualSignedMaterialAndRetainsConflicts`。
- `effectevidence/TestActualFileWriteAndFakeSuccess`。
- `effectevidence/TestFileCaptureBoundsAndUnsafeTargets`。
- `effectevidence/TestPendingFileImmutableRestartAndTamper`。
- `effectevidence/TestRecoveryConcurrentDifferentOwnersHaveOneWinner`。
- `effectevidence/TestRecoveryStoredCapacityAndFailureIsolation`。
- `effectevidence/TestRecoveryMalformedStorageFailsClosed` 的 gap、unknown-field、symlink、oversized、unpublished-temp 子例。

这些事件均实际返回 `file_observation_unavailable`。`effectevidence/file.go:45–51` 先接受本机 `filepath.IsAbs/Clean` 路径，再要求 `runtimeaction.NormalizeResource("filesystem", path)` 成功且结果原样相等；`runtimeaction/resources.go:42–46` 使用 POSIX `path.IsAbs` 并拒绝反斜杠。这是 Windows 真实文件路径无法进入产品观察链的实现缺口，不能归因于换行或临时目录不存在。

`completion/evaluate_test.go:24–27` 在最初的 `CaptureFile` 就失败，尚未写入预期文件、签发观察材料或验证完成冲突。恢复/篡改测试也需要先有合法基准文件快照；其当前名称不能替代实际到达的阶段。已知失败必须保留，后续更深层断言仍未验证。

协调入口继续使用已有 [Issue #39](https://github.com/maoyadongsh/siq-agent-security/issues/39)，不新建重复问题。建议 GLM/共享核心负责人先统一 Windows 资源合同、规范化、摘要与匹配，再同步 Intent、动作描述与 FileObservation；Windows 负责人在同一新候选复验本组和 Hermes 正常/越权真实调用。保持相对/盘符相对及未定义资源的拒绝，覆盖大小写、分隔符、边界、中文/空格、点段、UNC/设备前缀/ADS 等支持边界，不用 regex 或虚构 POSIX 路径把请求变绿。

### 3.2 可选 Connector 原生入口：实现与 fixture 双重假设

正式失败为 `inventory/TestConnectorsMergeWithoutDroppingNative` 和 `TestConnectorsNetworkAccessSkipped`。实际得到的是 `connector_missing`；后一例期望到达 `connector_failed` 的 network_access 拒绝，但尚未启动 Connector。

`inventory/connectors.go:81–94` 的查找候选没有 `.exe` 分支，并无条件要求 POSIX `Mode()&0111 != 0`。`inventory_test.go:282` 与 `:316` 写入无扩展名的 Python shebang fixture，以 0755 表示可执行。Windows 同时缺少合适的原生入口模型与等价测试制品，不能只解释成“缺 Python”，更不能把未运行的 Connector 认作已正确拒绝网络能力。

建议 GLM/Connector 负责人明确可信 Windows 可执行制品的查找与调用方式，保留普通文件/重解析点、摘要、输出上限、超时及网络能力拒绝。Windows 负责人使用已审阅的原生 fixture，复验发现→执行→合并 declared/禁止 effective→network_access 拒绝链路，并证明没有破坏原生发现结果。安装软件或扩大 PATH 不能替代该实现验证。

### 3.3 Runtime check 与 HTTP 文件观察受路径前置失败阻断

正式运行的 `runtimecheck` 7 个顶层失败为：`TestRuntimeCheckContractFixtures`、`TestCheckTemporaryAuthorityAndVerifiedCleanup`、`TestCancelStopsLaunchAndRevokesAuthority`、`TestRecoveryFindsBindingPublishedBeforeJournalUpdate`、`TestAuditFailureNeverStartsHostOrReportsPass`、`TestCleanupRefusesSymlinkAndCanBeRetriedAfterRepair`、`TestTamperedOrTrailingRecordCannotBeReadAsPassed`。主要实际诊断是 `runtime_check_intent_failed`、`launch not reached`、`incoherent runtime outputs`。

`runtimecheck/authority.go:26,96–98` 将本机 `filepath.Join` 的 allowed 路径创建为 Intent filesystem prefix，失败则返回 `runtime_check_intent_failed`。合同 fixture 在 `contracts_test.go:32` 先要求结果 passed 与真实 attachment 一致，当前未到最终样例比较，不能当纯 CRLF drift。

`recovery_test.go` 的 tamper 子例没有检查 awaitResult 必须 passed，也没有检查替换 `runtime_check_passed` 后字节必须变化；在已观察到的前置失败下，“corrupt evidence accepted”尚不足以证明完整性绕过。应先建立合法基准，再证实篡改确实发生并验证拒绝。

`server/TestFileObservationHTTPReadsRealState/{block,warn}` 与 `TestToolSuccessConflictsWithIndependentMissingOutputHTTP` 均在 `/v1/intents` 实际返回 400 / `intent_invalid_resource_constraint`，没有到达后续工具成功与独立文件观察冲突检查。以上沿 Issue #39 由 GLM 主修；Windows 复验时保留原命名用例的完整深层断言。

### 3.4 Windows 制品 staging 与 Python 目标识别

`skillmanifest/TestFetchAndStage`、`TestStageVerifiedBinaryHappyPath`、`TestStageVerifiedBinarySelfConsistencyWithoutPin` 正式返回 `stage source is not executable`。`stage.go:30–31` 在产品路径无条件要求 POSIX executable 位，这是 Windows 原生制品 staging 的能力缺口。应定义 Windows 可信可执行制品的判断与发布方式，保留常规文件/重解析点/摘要/源目标身份检查，不直接删除执行身份检查。

`TestPythonFetchArtifact` 正式退出子进程状态 3，报告 `no artifact for windows/`。`verify_manifest.py:97–101` 直接将 `platform.machine()` 归一为架构；本批另一个原生 Python probe 同样观察到空 machine 值，PE 文件头则独立证明 amd64。该日志足以证明当前环境的目标选择失败，尚不能认定 Python3 入口和环境缺失的唯一原因。GLM 应补可信架构识别与拒绝未知目标的设计，Windows 复验真实解释器、PE/OS 架构和 manifest 选择；不能写死 amd64 或改签名目标掩盖空值。

### 3.5 崩溃后 Writer 接管与迁移恢复

`state/TestAcquireWriterBlocksSecondAndTakesOverDead` 以及 `TestCommitRecoveryAfterProcessKill` 的 prepared/policy/audit/grant/done 子例均正式返回 Writer 仍被另一进程持有。`writer.go` 的 `processAlive` 明确在 Windows 无法证明进程死亡时保守返回 true。这是异常终止后恢复能力未接通的真实拒绝，未观察到错误抢锁；不能删锁、伪造死亡或以正常 stop 的成功替代 crash recovery。

`TestMigrationFullBackupAndEveryCheckpointRecovery` 的 backup/before-marker/marker，以及 `TestMigrationRecoversMissingMarkerOnlyAfterPreparedBackup/before-marker`，均实际返回 `state-migrate: existing output differs`。`migration.go:133–136` 将“读取/字节/模式任一不同”合并为该错误，并严格比较 POSIX mode；源码确实存在 Windows 模式等价问题，但本日志没有逐字段差值，不能宣称所有失败只由 mode 引起。产品在合法中断恢复场景拒绝继续是真实缺口；字节与权限何者触发仍需定点证据。

GLM/共享状态负责人主修死亡证明、恢复归属与 Windows 元数据合同；Windows 负责人复验旧/新状态、完整备份、每个检查点、已撤销授权保持、异常进程退出和真实权限。不得放宽未来状态屏障、既有输出一致性或未知所有权来恢复表面通过。

## 4. 临时目录与进程环境的只读核对

- 正式日志的实际失败路径位于本批独立 NTFS 临时目录，含 Windows 盘符与反斜杠。已只读确认该根存在且不是 reparse point；完整祖先路径含空格，没有非 ASCII 字符。
- 诊断时当前进程的 TMP/TEMP 均存在，系统临时路径为完整绝对路径；它与正式 Go 测试临时根不同。**诊断进程的环境不能冒充正式 Go 测试进程的环境**，本文未据此修改测试环境或推导 Go 使用了默认 TMP。
- 本机 Go 源码 `os/file_windows.go:284` 通过 Windows `GetTempPath`/`GetTempPath2` 取得临时目录；相关用例使用 `t.TempDir()` 和 `filepath.Join`。把临时根换成另一条合法 Windows 路径仍不能满足 POSIX `path.IsAbs` 或“禁止反斜杠”的资源规则。
- 没有本批证据显示这些已归类失败由临时根不可写、目录不存在或 Unicode 截断造成。空格路径的参数正确性需要按相关 CLI/子进程用例单独验证，不从根目录含空格推断所有错误均是 quoting。

## 5. 超时与仍未定因的正式失败

### 5.1 两个明确的包级超时

| 包 | 实际 panic 预算 | 包 elapsed | panic 时运行的末用例 | 该用例已运行时间 |
| --- | --- | ---: | --- | --- |
| `internal/server` | 10m0s | 602.672 秒 | `TestSkillUpdateCheckHTTPGuardsLeaveStateUntouched` | 9 秒 |
| `internal/skillinstall` | 10m0s | 613.869 秒 | `TestInstalledRuntimeActivationStrictAuthority` | 3 秒 |

两项末用例都有 run 事件但没有 pass/fail/skip 终态，已从成功计数排除。它们不是“自身卡了十分钟”的证据。`internal/state` elapsed 为 **585.127 秒**，有正常包级 fail 和明确测试断言，**没有 timeout panic**；`cmd/agentshield` elapsed 361.026 秒也未超时。历史其他超时包不计入本轮。

GLM/相应模块与 Windows 负责人应在同候选、相同预算下减少包间并发或独立测量，保留内部并发、容量边界和安全断言。当前只证明包总预算耗尽，未证明永久死锁、机器性能不足或 Defender 干预；不能先扩大预算或删慢用例洗绿。

### 5.2 其他需要定点证据的失败

| 正式失败 | 当前能证明的事实 | 未定事实与下一步 |
| --- | --- | --- |
| `receipt/TestStageTimingFollowsExecutedBranchesAndPreservesDecisions/{optional,required}` | authority_validation 等阶段各有一个样本但为 0s，fsync 有非零耗时；测试 `timing_test.go:32` 要求严格大于 0。`engine.go:1224–1225` 使用真实 `time.Now/time.Since`，不是注入的 authority clock。前后的安全决策一致性断言已到达并通过 | 更像短阶段计时分辨率/严格正数假设，尚未单独量化时钟精度；不声称丢了样本或改变安全决策。GLM 评测负责人校准可验证计时定义，Windows 保留分支分母与决策相同断言 |
| server 的 `TestInstanceAndPlanV2ContractFixtures`、`TestDiscoveryContractFixtures`、`TestGrantDraftContractSamples`、`TestManagedPlanV3ContractFixture`；skillimport 的 `TestRemoteImportContractSamples` | 五个顶层用例确实发生合同样例比较差异；日志只给文件/用例名 | 当前没有字段级差值，不能继续一概归因旧 CRLF，也不能认定所有差异只是 Windows 路径。GLM/合同负责人先取得脱敏结构差异，再判断环境投影、实现或样例是否应变；禁止打开样例更新开关直接覆盖原签名/基线 |
| `skillimport/TestGitImportClonesFixedCopyWithoutHooks`、`TestGitImportSubDirectorySelectsSubtree`、`TestUpstreamCheckGitSeesAdvancedHeadWithoutTouchingRecord` | 均在本地 file:// fixture 的 Git 获取阶段报 `skill_import_download_failed`，尚未走到内容可执行位或上游更新断言。`git.go:218–232` 将非零 Git 进程错误映射为通用错误并丢弃 stderr | 具体是 URI、参数、隔离环境或其他 Git 失败尚未知，不能复制原开发日志的“executable bit lost”分类。GLM 安全 Git 负责人用受控本地 fixture 定点核对 argv/退出阶段，诊断不得归档远端可控原始 stderr；Windows 复跑固定副本/禁 hook/commit pin/更新记录不变 |
| `skillmanifest/TestAdapterAndBootstrapShareVerifiedResolve` | 正式仍报期望 hash/verify failure，但实际捕获内容为空 | shell 入口、参数路径与实际拒绝阶段不明；不能把空输出称为哈希绕过。后续只读确认解释器身份，再用受控 fixture 留退出码与脱敏阶段 |
| `state/TestIndependentProcessesPublishUniqueVersions` | 4 个 worker 均返回退出 1；期望 80 个版本，实得 47；顶层 elapsed 20.98 秒。`versions_test.go` 以 20 秒共享 context 启动这些 worker | 与用例自身预算耗尽相符，但日志未给 worker 具体失败原因，不能宣称已证实超时根因。80 是计划提交数，不是已确认成功提交数，不能据此声称丢失了 33 次已成功写入。GLM/状态负责人记录每个已确认提交与取消阶段，Windows 独立测量并发/文件系统延迟；不加入两个包级 timeout 数 |

以上 unknown 不等于忽略失败；相关用例与包仍计为 fail。所有未知原因、未运行的深层断言和超时中断均阻止全量/平台总体通过声明。

## 6. 后续主修与复验安排

| 工作包 | 建议主修 | Windows 复验责任与完成条件 |
| --- | --- | --- |
| 资源合同、Intent、动作与文件观察 | GLM / 共享核心负责人，沿 Issue #39 | 正式路径正负例、上述 completion/effectevidence 深层断言、Hermes 原生受限允许/越权拒绝与最终参数一致性；不得只过 API |
| 跨 OS fixture 与序列化 | GLM / 对应测试负责人 | 用正确 OS/编码输入跑原断言；明确哪些是纯组件、哪些必须在目标 OS 原生跑，保留原失败和替代验证理由 |
| Connector 原生制品入口 | GLM / Connector 负责人 | 使用真实可执行 fixture 到达 discovery/exec/能力拒绝断言，核验 declared/effective 分离及负例副作用 |
| Windows 文件共享、链接与 ACL | GLM / 共享文件状态负责人，与 Windows 平台负责人共同定义 | 攻击构造本身可证明、DACL/身份和副作用可核验；私有测试根权限不能代替每个产品文件/备份的验证 |
| Writer 死亡证明、迁移元数据、Windows staging 与安装归属 | GLM / 对应共享模块负责人 | 沿第 2、3 节的真实失败建立原生恢复/制品/权限负例，保留未知对象与历史字节 |
| 两个包级超时、合同差异、Git/shell/计时未知 | GLM / 相应模块与评测负责人 | 以最终实际事件定点诊断，记录同候选隔离测量；不改 skip、不缩小原负例或容量分母 |

以上是协调建议，未替 GLM 或其他维护者承诺排期。最终失败包及其有终态的顶层失败测试名均在 `go-final-summary.json`；该 JSON 不包含原始私有路径或启动日志。部分 Windows 子批和本批专项有明确通过范围，但全量 Go 退出 1，Windows 平台总体验收与 N09 不关闭。

## 7. 失败包完整索引

| 包（省略共同模块前缀） | 顶层 fail 数 | 主要分类 | 包级 timeout |
| --- | ---: | --- | --- |
| `cmd/agentshield` | 9 | LaunchAgent/systemd 路径与 symlink 构造 | 否 |
| `internal/adapterinstall` | 1 | 未安装 wrapper 的存在性断言 | 否 |
| `internal/adapters` | 1 | Windows 路径未转义的 JSON fixture | 否 |
| `internal/admission` | 1 | 持有文件后的替换构造 | 否 |
| `internal/completion` | 1 | 文件资源路径前置失败 | 否 |
| `internal/effectevidence` | 7 | 文件资源路径与 symlink 构造 | 否 |
| `internal/inventory` | 3 | Connector 原生入口与 symlink 构造 | 否 |
| `internal/pending` | 1 | POSIX mode 断言 | 否 |
| `internal/receipt` | 1 | 0s 计时样本的严格正数断言 | 否 |
| `internal/runtimecheck` | 7 | Intent 路径前置失败、下游断言未到达 | 否 |
| `internal/runtimeidentity` | 2 | POSIX mode/权限攻击与 symlink 构造 | 否 |
| `internal/server` | 6 | 四项合同比较、两项路径前置失败；另有未终态测试 | 是 |
| `internal/skillimport` | 6 | Git 获取未知、合同比较、mode 与 symlink 构造 | 否 |
| `internal/skillinstall` | 2 | symlink 构造、case-alias 人工恢复；另有未终态测试 | 是 |
| `internal/skillmanifest` | 7 | staging/架构能力、mode/symlink 与 shell 未知 | 否 |
| `internal/state` | 6 | Writer 接管、迁移恢复、mode 与 worker 预算/失败未知 | 否 |
