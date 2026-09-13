# SIQ Agent Security：共享协作与验收规则

> 版本：1.0；编制时间：2026-09-13 20:23:55（Asia/Shanghai）。
> 事实基线：`origin/main` / `0720730f821373b00bb8bfd5ec870fec147e5746`，包含 N01 修复 PR #35、后续任务书 PR #36。
> 执行人：GLM 开发窗口、sunbo 的 Windows Codex 窗口、Luke-zzZ-0 的 macOS Codex 窗口；仓库：`maoyadongsh/siq-agent-security`。
> 本文规定协作和证据要求，不声明 Windows/macOS 已验证，不改变运行时权限合同，不启动 LAN 阶段。

## 1. 使用方式与目标

本文件与 [sunbo Windows 任务书](personal-windows-sunbo-taskbook-20260913-202355.md)、[Luke macOS 任务书](personal-macos-luke-taskbook-20260913-202355.md)共同使用。每位协作者把本规则和自己的任务书交给本机 Codex。先完成环境盘点，再实测、定位、修复、回归并提交 PR，不止交一份截图或问题列表。

总目标继承 [任务书 v3.0](personal-experience-lan-team-next-development-taskbook-20260913-192253.md)和[原始范围](personal-experience-lan-team-development-taskbook-20260910-145507.md)：三系统上的 OpenClaw、Hermes、WorkBuddy，发现已有资产、确认权限后接入、受控执行、安全安装与确认更新、统一审批、脱敏追溯与可恢复生命周期。保留用户原平台界面；先个人 N09 验收，后局域网 T01–T06。

已确认：sunbo 负责 Windows，Luke-zzZ-0 负责 macOS；两位均承担实测、定位、系统适配修复及 PR。设备 OS 版本、CPU、智能体安装情况目前未知，由各自 P00 获取。不得预填 Win11/x64、Apple Silicon、平台发行版或“已安装”。

## 2. 当前事实与阅读顺序

| 已核对内容 | 当前结论 | 后续执行要求 |
| --- | --- | --- |
| N00/N01 | 已核查旧分支；N01 代码与 Linux 最低门槛完成 | Windows/macOS 原生行为仍由本次实测，不重写 N01 |
| Windows 生命周期 | 已有 task 准备、注册、启动、状态、停止、注销及 setup 组合代码 | 使用实际 Task Scheduler、进程、健康身份和副作用验收 |
| macOS 生命周期 | 已有 LaunchAgent 配置、注册、加载、启动、停止、注销及 setup 组合代码 | 在真实当前 GUI 用户域验证，不把 mock 当系统行为 |
| Git 获取 | 本基线 `fetchGitCLI` 明确返回 `ErrGitTransportUnavailable` | GLM 负责安全生产路径；未验收前不放开 clone |
| 更新 | M132 手动只读检查已有，自动检查在接续计划中 | 区分“检查成功”与“用户确认安装”；不静默更新 |
| 平台接入 | Hermes/OpenClaw 有部分 Linux 原生及组件证据 | Windows/macOS 重新测；未知归属仍 unknown |
| WorkBuddy | `connectors/workbuddy` 是发现入口 | 不拿 CodeBuddy 或 Connector 单测代替 WorkBuddy 桌面执行 |
| 默认通知 | `internal/notify/notify.go` 在该基线仅探测 Linux notify-send | 两系统默认通知与点击导航需要实现/实际验证 |
| Web 开发脚本 | `dev:local` / `build:local` 使用 POSIX 环境赋值语法 | Windows 原生 shell 兼容是待验证/优化项；见 Windows 任务书 |
| 平台校验器 | 已有 18 行候选合同和 init/verify 工具 | 校验材料结构/摘要；不证明正文真实或产品 supported |

GLM 正在其他工作树开发，其未合并内容不计为本基线已完成。每批开始 fetch 并检查新提交、当前台账、相关 PR；更新这张表的差异记录，不照抄历史缺口，也不依据聊天报告推定代码已合入。

阅读顺序：适用 `AGENTS.md` → 总任务书 → [本地规格](agentshield-dev-spec-v1.md)及相关 ADR → [N01 规格](n01-state-protocol-design-20260913.md) → [平台材料规格](personal-platform-validation-spec-v1.md)与 `packages/contracts/` → 对应实现和测试。历史 README 中的版本/超时只说明当时证据，不能替代当前宿主探测。来源冲突先记录并统一规格，不能只改验收结果。

## 3. 分工、共享文件与问题归属

| 角色 | 可以直接推进 | 必须协调的变更 | 复验责任 |
| --- | --- | --- | --- |
| GLM | 共享状态/授权/下载/调度/API/UI 核心，Linux 基准闭环 | 改变三系统行为、平台钩子合同、状态格式、签名字段 | 核心正负向及 Linux；提供候选 SHA 和兼容说明 |
| sunbo | Windows 环境、原生/WSL2 分别实测、系统适配、脚本、证据 | 公共核心、跨平台适配器协议、共有 npm 脚本/Schema | Windows 本机复验和受影响共享回归 |
| Luke | macOS 环境、真实 GUI/LaunchAgent、系统适配、脚本、证据 | 同上；共享通知接口由双方先约定，不能各造协议 | macOS 本机复验和受影响共享回归 |
| 仓库维护者/指定审阅者 | 分配公共问题、审阅证据、合并和范围决策 | 平台范围缩减、正式发布、治理变更 | 核对最终候选、缺口与跨平台回归 |

所有权是协作约定，不是禁止平台开发者修复公共代码。公共问题用一个问题编号指定主修人；平台负责人交最小复现、预期行为和补丁建议，主修人实施或明确交由平台负责人实施，其他人避免同时修改同一路径。同一漏洞不各修一个 Windows/macOS/Linux 分支版本。

高冲突路径：`packages/contracts/`、`internal/state*`、`grant`、`receipt`、`runtimeidentity`、`skillimport`、`skillinstall`、`clientrelease`、`server`、两个运行时适配器、`apps/web/package*.json`、共享 API 类型及总台账。OS 适配沿既有平台文件/接口扩展，不复制决策引擎。`internal/notify` 的通用调度/隐私接口先统一，再分别实现平台投递。

任务书要求留下 Issue/PR 可审阅记录；工具向他人发送邮件、即时消息或自动 @ 催办不属于这份任务的默认授权。由协作者通过正常仓库协作界面交接，不把“等待协调”变成停止所有独立工作。

### 3.1 问题路由

- 某 OS 独有：平台负责人修复，例如 XML/plist 引号、路径、系统管理器状态读取。
- 三系统共用：一个主修人处理，例如状态兼容拒写遗漏、审批复用、Git 网络策略。
- 宿主不提供能力：记录官方资料和实机结果，提出原生替代/受控启动方案；未知保持未知，不能配置一个能力标记伪装支持。
- 缺设备/账号：阻塞具体子项，先完成其他平台或组件工作。缺实机不是代码验收失败；实机观察到错误必须记 fail，不能改 blocked 掩盖。
- 越权执行、凭据泄漏、数据损坏、撤权复活：优先修复和回归；不用“测试环境问题”绕过。

## 4. 工作批次与依赖

三条线现在即可并行开展；这里的并行是不同协作者工作，不要求每个 Codex 自动启动子代理。

| 阶段 | 本机交付 | 映射 | 是否等待 GLM |
| --- | --- | --- | --- |
| P00 | OS/CPU/运行方式/版本与隔离资源盘点 | N00/N04-A | 不等待 |
| P01 | 构建、启动、配对、管理恢复与初始发现 | N07/N08 | 已有基础可测；发现缺陷自行路由 |
| P02 | 三宿主逐个发现、接入、正常/越权/失联、还原 | N04，关联 N05/N06 | 已有路径先测，缺核心能力的单项登记依赖 |
| P03 | OS 生命周期、文件安全、N01 迁移/回退预检 | N01/N07 | 不等待无关 Git/调度功能 |
| P04 | 审批继续、通知、安装更新、可信归属、追溯 | N03/N05/N06/N08 | 依赖对应已实现候选；准备复现和脚本可先行 |
| P05 | 完整用户旅程、干净候选、PR 与平台验收报告 | N07/N09 | 全部必需项有证据；缺项不得关闭平台总体 |

Win-Pxx/Mac-Pxx 是现有 N/UX 工作包的执行子批次，不替换总编号。每一 P02/P04 可按平台拆成 OpenClaw、Hermes、WorkBuddy 三批；一批应有一个可验证的行为结果，避免只积累 API 或大包补丁。

## 5. 基线、分支、候选和 PR

1. 每人在自己的机器克隆同一仓库或维护 fork。先检查 working tree/index，禁止 reset/clean 掉已有工作。从最新上游 main 新建短分支，例如 `codex/windows-sunbo-p01-<timestamp>`、`codex/macos-luke-p02-hermes-<timestamp>`。
2. `origin` 若指个人 fork，应显式设置/使用指向 `maoyadongsh/siq-agent-security` 的上游；所有“最新 main”均指该仓库，不凭远端名称判断。检查 URL 不输出带 token 的地址。
3. 每批测试固定一个代码 SHA、宿主版本及二进制摘要；测到一半不要 pull。修复后建立新候选，原失败记录保留。使用本地提交固定候选并不等于发布版本。
4. 正式平台材料要求 `candidate_sha` 为 40 位 SHA、`source_dirty=false`。先提交实现/测试、在干净候选构建并实测，再单独提交证据。开发中的脏树结果写临时复现报告，不伪填 clean。
5. Go embed 和适配器嵌入资源属于实际运行内容。前端/适配器变更先按现有构建机制同步资产，再固定候选；若构建导致 tracked 文件变化，审阅并纳入候选后重新确认所测内容，不能测旧 embed。
6. PR 指明代码候选 SHA、证据提交、最终 PR head。仅附证据的后继提交可引用相同实现候选；若最终 head 修改了运行内容或解决了行为冲突，受影响实机项目必须重测。不同 SHA 的材料不能改成同一个 candidate 伪造全矩阵。
7. 本轮用户已明确两位以 PR 提交修复。按此角色授权完成本地提交、推送自己的分支并创建审阅 PR；不直接写 main，不自动批准/合并自己的 PR、不改治理、不发布安装包/标签。仓库维护者另行执行合并。
8. 同一个 PR 聚焦一个问题或连贯子批次，合同用 `contracts:` 单独提交，其余按 AGENTS scope。不要带私有配置、node_modules、venv、用户数据或二进制。PR 模板和检查要求以实际仓库为准。

## 6. 安全与用户体验共同底线

- 新持久化通过 N01 的 `stateformat/statefs` 和现有 Writer 边界；不绕过检查、不删除迁移屏障、不用 init 冒充迁移。版本不兼容必须在有副作用前拒绝。
- 私钥仅在本地私有状态；管理员 token 不进 URL、浏览器持久化存储、通知和证据。展示路径/URL、用户名/SID、命令原文前脱敏。
- `declared/inferred/observed/effective` 分开，effective 只来自真实执行后端读回。无效必需 Authority 所有模式 hard deny；block 模式失联、超时、401/非法返回拒绝。
- 被扫描 Skill 不执行、导入或 eval。真实执行测试只用自建、已审阅的无害 fixture 和隔离资源，不能运行未知下载内容“试试是否安全”。
- 同 UID 不构成 OS 沙箱。不能因路径检查、成功安装或“无风险”扫描就把 UI 标为全面保护。
- 可信 Skill 归属不能来自自报 ID、cwd 或文件名；撤销、换版本、换会话/参数后旧授权不得复用。审批 HTTP 成功不等于执行成功或仅执行一次。
- 发现和预览阶段不改用户配置；安装/接入、授权、更新、迁移、卸载走实际确认路径。后台操作不自动扩大保护或权限。
- 平台测试只使用明确命名的独立 SIQ 状态和智能体 profile/测试用户。系统服务测试按实例归属操作，不能停其他进程、广泛删除任务/LaunchAgents、关闭防护或永久改变机器策略。
- 用户级服务、系统通知和登录/重启需要真实会话观察。权限拒绝应产生可恢复提示，不能把要求管理员/root 作为默认修复。
- 不默认调用付费模型；可用已有受控模型 fixture 验证原生宿主工具链，但必须标“原生宿主 + 合成模型”，不能声称验证了真实云端模型/桌面交互。实际账号/费用需求由操作者提供明确范围。

## 7. 统一实测用例

每项保存输入/预期/实际、版本/摘要、命令或交互步骤、退出结果及副作用。不存在的能力保留缺口，不能临时改实现返回 true 来通过。

| 用例 | 操作和必须证明的结果 | 反例/恢复 | 关联 |
| --- | --- | --- | --- |
| A01 启动与身份 | 指定状态/端口启动，管理页显示正确实例 | 端口是其他服务、错状态、认证失效不能复用为成功 | N07/N08 |
| A02 发现 | 三宿主、多 profile、同名 Skill、非默认路径 | 未确认不写配置，不把仅发现标成已保护 | N04 |
| A03 接入与还原 | 预览→确认→备份→安装→真实宿主加载→自检 | 配置漂移/冲突拒绝；仅移除本产品登记，保留用户内容 | N04 |
| A04 真实允许/拒绝 | 已批准的限定读取成功；越权写入被宿主阻止 | 检查哨兵文件/测试接收端，无真实副作用；不能仅看 deny 日志 | N04/N05 |
| A05 服务失联 | 已接入 block 模式停 SIQ/受控超时，重新调用 | 不执行；恢复正确服务后按有效授权恢复，不能默认 allow | N04 |
| A06 审批/最终复验 | 待确认不执行、批准一次后正确执行 | 拒绝/过期/撤销/改参/换版本/响应丢失/重复回调不绕过或重复副作用 | N06 |
| A07 Skill 归属 | 从可信宿主链绑定实例、会话、安装版本与调用 | 自报、跨实例、同名/同内容不同安装不借权；不支持时 unknown | N05 |
| A08 安装/更新/移除 | 本地/受支持 URL 预览、确认、识别、重启验证 | 未批准目标不变；候选漂移拒绝；卸载不复活授权或删除未知对象 | N02/N03/N04 |
| A09 通知 | 实际桌面投递，点击只打开正确本地待办 | 无 token/参数原文，不能点击直接批准；拒通知时 inbox 可用 | N06/N07 |
| A10 状态与发行 | v1 受支持实例→v2；签名制品兼容预检 | 未来格式拒写、每类中断恢复、撤销保持、业务摘要不变 | N01/N07 |
| A11 活动与隐私 | 主体/版本或 unknown、授权、决策、结果正确关联 | 有回执不等于成功；原文默认关，导出/过期/撤销不泄漏 | N08 |
| A12 完整生命周期 | 用户级安装→管理→登录/重启→升级恢复→卸载 | 默认保留数据，不破坏他人配置；缺登录触发器明确缺口 | N07/N09 |

N01 边界：仅支持已初始化 v1/已识别无标记 → v2 元数据转换；真实备份含密钥，严禁上传。已测旧程序是兼容感知 v1 构建，不是任意历史发行；没有可信旧程序就记录该项 blocked，不把新程序换版本名当旧程序。当前预算 10,000 条目、128 MiB/文件、2 GiB 总内容，POSIX mode 不能证明 Windows ACL/macOS ACL/xattr 保存完整。系统差异必须显式验证和设计，不放宽来源/归属规则。

## 8. 证据合同：避免“单平台无法全绿”的误用

事实源：`packages/contracts/personal-platform-acceptance.v1.schema.json`、对应 report schema 和 `scripts/personal-experience/platform_acceptance.py`。

- 全部 18 行必须保留：每平台 Windows amd64 native/WSL2、macOS arm64/amd64 native、Linux amd64/arm64 native。它是候选清单，不声称所有运行形态存在。设备不在清单（如 Windows ARM）时另存探索证据，提出合同/范围增量，不能把 arm64 填为 amd64。
- Windows 原生不等于 WSL2。macOS Apple Silicon、Intel 与 Rosetta 路径分别说明实际机器、进程与二进制架构；不把一台机器模拟/翻译执行当成另一台原生验收。
- 八项检查：discovery、normal_execution、pre_execution_denial、service_unavailable_denial、approval_resume、final_parameter_recheck、skill_attribution、install_interception。完整生命周期/通知/迁移等用本批报告补充，不往严格 Schema 擅自加字段。
- 状态只用 not_run/blocked/pass/fail；method 只用 none/source_review/component_fixture/native_cli/native_desktop。WorkBuddy 的目标材料强度为它本身的 native_desktop，不能换成 CodeBuddy 或网页夹具。
- 未测版本为 null。pass/fail 要有实际版本、二进制摘要和引用；pass reason=null，fail reason=observed_failure，blocked 使用合同允许原因并在报告解释。不要自创 enum。
- 每人新建完整 manifest，填自己真正测过的行，其余保持未测。普通 verify 的 0 仅说明材料结构/摘要一致；`--require-native` 仍有缺口应退出 3，这是部分平台交付的预期，不是要删除其他行的信号。非法材料退出 2 必须修复。
- 工具不自动创建父目录、不覆盖 `--out`；每次采用新文件名。允许证据扩展名为 .json/.txt/.log/.png；Markdown 报告可以归档，但不能直接作为此合同的证据引用。
- manifest ≤512 KiB，每检查最多 4 个引用，每证据 ≤8 MiB，总独立读取 ≤32 MiB；路径相对且稳定，拒绝软/硬链接、reparse、设备名/ADS 和特殊文件。截图先脱敏，录像在本机私有留存或受控位置另索引，不能硬塞进合同绕预算。
- 校验器不能检测所有秘密或证明日志真实。人工检查副作用与链路；同一证据内容不跨组合复用，合成全绿样例不是验收。

各平台材料先独立验证。最终总矩阵必须由维护者在同一受验收候选上汇总受影响项目的重测结果；不直接拼接不同 candidate 的 JSON 或改 SHA。某平台未完成不阻止另一平台有证据的小修复 PR 合并，但 N09 仍不关闭。

### 8.1 目录与证据身份

本机原始材料放在 repo 外的私有测试根；脱敏后归档到 `docs/evidence/personal-experience/windows-sunbo/<batch>-<timestamp>/` 或 `macos-luke/<batch>-<timestamp>/`。建议包含：

| 文件 | 内容 |
| --- | --- |
| report.md | 本批问题/最终行为、Axx/Nxx 映射、未测/限制、恢复清理、用户旅程 |
| environment.md | OS/CPU/宿主与工具版本、原生/WSL/GUI 方法、官方来源日期；不含机器敏感标识 |
| verification.json | 沿仓库已有证据约定记录候选、命令、退出码、材料类别、摘要；不是 runtime 权限协议 |
| matrix.json / matrix-validation.json | 严格遵守现有 18 行合同和校验器输出 |
| evidence/*.json/.log/.png | 经过脱敏的可引用实际证据 |

本表是材料组织约定，不新增生产 JSON 合同。未经测试的空模板只能标 not_run。原始失败日志/旧 SHA 保留，不覆盖已归档结果。

## 9. 验证与合并门槛

最低命令按修改范围和 AGENTS.md；所有命令记录实际退出码，测试输出中的“PASS”字符串不能替代进程结果：

- Go：gofmt、`go vet ./...`、`go test ./...`；并发/状态/授权改动跑相关 race。本机缺 race 工具链记录原因，并使用受信 CI 补充，不宣称已跑。
- 至少保留 linux amd64/arm64、darwin arm64、windows amd64 构建；Intel Mac/其他架构实测须构建对应候选，交叉构建不算原生。
- Web：类型检查、测试、企业和本地构建；Windows 原生命令见任务书，不把 WSL 产物误记原生脚本验证。
- 合同/共享 Python：复用 `apps/control-api` 锁定环境，Schema 样例、兼容、适配器负向；变更规则时遵守 Python/Go 共享语料要求。
- OS 行为：至少相关真实宿主和真实系统管理器复验；公共修复合入前补 Linux/另一 OS 的受影响回归证据或明确由谁在候选上完成。
- PR 最终 head 的既有 CI 通过；三 OS CI 中的 evidence-tool 是校验器单测，不能据此关闭原生矩阵。安全负向失败不能通过扩大忽略/删测试解决。

平台子批可 verified；平台整体验收 accepted 需独立审阅者核查本机证据和全部必需用户旅程。只有测试者自己声明“通过”不关闭 N09；若无独立同 OS 复验环境，明确区分证据审阅与独立重跑，不伪称两者都完成。

## 10. 开发窗口启动指令与批次模板

> 你在 SIQ Agent Security 仓库负责指定 OS 的真实环境开发。先读本规则和对应平台任务书，从最新上游 main 的独立分支开始。P00 获取未知设备和三个宿主信息，随后实测已有能力、复现并修复本系统适配问题，通过 PR 交付。公共核心与 GLM 协调主修人，继续不依赖阻塞的本机任务。保持状态兼容、授权硬拒绝、unknown 归属、一次性审批、通知隐私和配置归属规则；不修改日常实例或降低系统防护。所有完成声明绑定候选 SHA、实际二进制和真实副作用证据；18 行候选保留，部分平台 require-native 返回 3 如实登记。不自动合并、发布、修改治理或发送外部消息。

```text
负责人 / Win-Pxx 或 Mac-Pxx / 关联 Nxx、UX：
上游 main SHA / 分支 / 受测实现 SHA / PR head：
OS、CPU、进程架构、运行方式、平台和工具版本：
本批问题、用户可见的最终行为、改动路径：
实际命令/退出码、Axx 用例、原生或组件证据路径：
负向拒绝及无副作用证明 / 恢复 / 清理结果：
未测、失败、阻塞、公共依赖及主修人：
下一批 / 平台总体是否满足验收门槛：
落盘、提交、推送、PR、合并、发布状态分别说明：
```
