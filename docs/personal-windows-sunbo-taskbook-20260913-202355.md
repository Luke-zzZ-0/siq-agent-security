# sunbo / Codex：Windows 真实环境实测与开发优化任务书

> 版本：1.0；时间：2026-09-13 20:23:55（Asia/Shanghai）。
> 负责人：sunbo；执行工具：sunbo 本机 Codex；目标仓库：`maoyadongsh/siq-agent-security`。
> 编制基线：`0720730f821373b00bb8bfd5ec870fec147e5746`；实际批次必须重新抓取上游并记录受测 SHA。
> 设备信息目前全部待确认；不得假设 Windows 版本、CPU、三个宿主已安装或均支持 Windows 原生。

## 1. 任务目标与第一步

把本项目已有个人能力在真实 Windows 环境中跑通，发现问题后负责定位、系统适配修复、回归与 PR。目标平台分别为 OpenClaw、Hermes、WorkBuddy；Windows 原生与 WSL2 分别验证。先形成可信的基线，再跟随共享核心补齐安装、审批、Skill 更新、追溯、后台生命周期与卸载，最终提交 Windows 平台验收材料。

本书与 [共享协作验收规则](personal-platform-collaboration-acceptance-20260913-202355.md)配套；总范围见 [v3.0](personal-experience-lan-team-next-development-taskbook-20260913-192253.md)。必须读取适用 AGENTS、[本地规格](agentshield-dev-spec-v1.md)、[状态协议](n01-state-protocol-design-20260913.md)、[平台材料规格](personal-platform-validation-spec-v1.md)。原 N/UX 编号和安全要求不变。

首批执行 **Win-P00 + Win-P01**。环境信息未知由本机检测与操作者确认补齐，不要先等待另一窗口提供。缺一个宿主不阻止其他宿主/生命周期测试。公共核心问题与 GLM 协调主修人；你保留 Windows 复现与复验责任。

### 1.1 已有实现：先复用，不重写

下列路径均相对仓库根目录，代表编制基线已有代码，不代表 Windows 实机通过：

| 能力 | 现有入口 | 本机重点 |
| --- | --- | --- |
| 启动/管理组合 | `apps/agentshield/cmd/agentshield/setup_windows_task.go`、`setup_launch_agent.go` 中共享组合、`main.go` | 当前用户 SID、系统管理器预检、启动后身份/健康读回 |
| Task Scheduler | `cmd/agentshield/windows_task*.go`、同目录 `windows_task*.ps1`、`internal/state/windows_task.go` | 使用既有绑定和归属，真实注册/启动/停止/注销 |
| 系统程序定位 | `cmd/agentshield/windows_task_exec_windows.go` | 系统目录定位，不能用 PATH 上同名恶意程序替代 |
| 文件/状态保护 | `internal/fileopen/`、`internal/stateformat/`、`internal/statefs/`、`internal/state/` | NTFS、重解析点、文件占用、ACL、迁移中断与零写入拒绝 |
| 发布/恢复 | `internal/clientrelease/`、`internal/skillmanifest/`、`cmd/agentshield/client_install.go` | 实际二进制摘要、v3 状态兼容声明与副作用前预检 |
| 平台资产/配置 | `internal/inventory/`、`internal/adapterinstall/`、`connectors/workbuddy/` | profile、Windows 路径、配置备份、真实运行加载 |
| 宿主桥 | `adapters/runtime/openclaw-agentshield/`、`adapters/runtime/hermes-agentshield/` | 真实 pre/post、会话/调用绑定、失联拒绝、最终执行复验 |
| 通知/审批 | `internal/notify/`、`internal/pending/`、`internal/runtimeidentity/` | Windows 默认通知该基线未实现；实际恢复语义独立验证 |
| 管理界面 | `apps/web/src/local/`、`internal/server/`、`internal/ui/embedded/` | 配对/会话恢复、目录选择、实际保护状态、中文错误与权限提示 |
| 验证工具 | `scripts/personal-experience/`、`scripts/test-openclaw-adapter.cjs` | 先读源码确认 OS 假设，不用 Linux 脚本结果代替 Windows |

表内 `cmd/…` / `internal/…` 缩写均位于 `apps/agentshield/`。Windows XML/注册等基础已合入 main；旧文档说“仅 XML 已实现”不再准确。安全 Git 和自动检查随 GLM 后续提交变化，每批核实。

## 2. Win-P00：设备、宿主与测试资源盘点

**交付：** `environment.md`、三个宿主的能力初表、当前代码/工具身份、缺口与首批计划；本阶段不改日常实例。

记录以下实际值，不获取序列号、完整个人用户名、许可证密钥或账号 token：

- Windows 版本/build、OS 架构、终端/PowerShell 版本与进程架构；Go 的 GOHOSTOS/GOHOSTARCH 和是否设置交叉编译环境变量。
- 文件系统及工作目录/测试状态目录：本地 NTFS、同步盘、网络盘等分开。确认本机测试目录 ACL 和可写性，不能认为 Go 的 mode 0600 已完成 NTFS 隔离。
- Git/Go/Node/npm/Python/uv 版本，系统浏览器版本；是否可以使用普通用户 Task Scheduler、实际交互桌面及系统通知。
- 每个宿主：是否安装、实际版本、来自哪里、可执行入口、profile/Skill 目录、原生或 WSL2、可否创建隔离测试实例。当前官方支持情况以官方文档与实机为准，记录 URL/日期；不能凭模型记忆填写。
- WSL2 若存在：Windows 版本、guest 发行版/内核/架构、宿主进程所在系统、SIQ 进程所在系统、网络与文件路径边界。Windows 浏览器能打开 WSL 页面不证明 Windows 原生 runtime。
- 明确可以使用的测试账号/profile、测试目录和端口。缺账号、桌面登录或允许重启的时段时，记录具体阻塞；不主动安装软件到用户生产配置。

可用的只读盘点命令，先确认工具存在，输出需脱敏后归档：

```powershell
$PSVersionTable.PSVersion
[System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
[System.Runtime.InteropServices.RuntimeInformation]::ProcessArchitecture
Get-CimInstance Win32_OperatingSystem | Select-Object Caption, Version, BuildNumber, OSArchitecture
Get-Command git, go, node, npm, python, uv -ErrorAction SilentlyContinue | Select-Object Name, Source
go version
go env GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED
node --version
npm.cmd --version
python --version
uv --version
# 仅当存在 WSL：检查清单；发行版名称含个人信息时脱敏，不创建/升级发行版。
wsl.exe --list --verbose
```

上述某工具缺失属于环境缺口，不把整批标 fail；真实宿主调用产生错误则按实际失败记。安装依赖仅针对开发工具和隔离环境，按官方来源、锁文件与操作者机器约束处理，不关闭 Defender、全局降低 ExecutionPolicy 或要求管理员权限来掩盖适配缺陷。

**P00 退出：** 能明确选择至少一个本机可测组合，其他组合有真实状态；若设备是 Windows ARM，不套合同 amd64 行，先登记探索项并协调候选合同增量。

## 3. Win-P01：干净候选、原生构建与首次管理

### 3.1 建立工作分支

在本机真实 repo 执行。不要使用仓库维护者 Linux 的 `/home/maoyd/...` 路径；个人 fork 要把下面的上游远端改为实际指向官方仓库的名字。

```powershell
$SiqRepo = (git rev-parse --show-toplevel).Trim()
if ($LASTEXITCODE -ne 0) { throw '当前目录不是目标 Git 仓库' }
Set-Location $SiqRepo
git status --short --branch
# working tree/index 有改动时先保留、隔离，不能 reset 或 clean。
git fetch origin --prune
if ($LASTEXITCODE -ne 0) { throw '无法获取上游' }
$SiqStamp = Get-Date -Format 'yyyyMMdd-HHmmss'
git switch -c "codex/windows-sunbo-p01-$SiqStamp" origin/main
if ($LASTEXITCODE -ne 0) { throw '创建分支失败，保留现场' }
git rev-parse HEAD
```

先确认 main 包含 N01。记录分支/main 差异，不检查出 GLM 未提交目录，不默认拉取未知二进制或覆盖本机程序。

### 3.2 Windows 原生前端与 Go 构建

本基线 npm `build:local` 的 `VITE_APP=agentshield vite build` 是 POSIX 写法，不能把它在默认 Windows shell 失败归因于业务 UI。以下是使用锁定本地工具的 Windows 等价构建步骤；它不是已验收新脚本。后续可提出无新增依赖的跨平台 npm 启动器修复 PR，并与 GLM 协调 package.json 的所有权，验证企业/local/dev 入口均不变。

```powershell
# 在仓库根；所有原生命令失败必须显式检查退出码。
Push-Location (Join-Path $SiqRepo 'apps/web')
$SiqPreviousApp = $env:VITE_APP
try {
    npm.cmd ci
    if ($LASTEXITCODE -ne 0) { throw 'npm ci 失败' }
    $env:VITE_APP = 'agentshield'
    node.exe node_modules/typescript/bin/tsc -b
    if ($LASTEXITCODE -ne 0) { throw 'TypeScript 检查失败' }
    node.exe node_modules/vite/bin/vite.js build
    if ($LASTEXITCODE -ne 0) { throw '本地前端构建失败' }
} finally {
    $env:VITE_APP = $SiqPreviousApp
    Pop-Location
}
$SiqBuildDir = Join-Path $SiqRepo ".tmp/personal-windows-$SiqStamp"
New-Item -ItemType Directory -Path $SiqBuildDir -ErrorAction Stop | Out-Null
$SiqBinary = Join-Path $SiqBuildDir 'siq-agent-security.exe'
Push-Location (Join-Path $SiqRepo 'apps/agentshield')
$SiqPreviousGoos = $env:GOOS
$SiqPreviousGoarch = $env:GOARCH
$SiqPreviousCgo = $env:CGO_ENABLED
try {
    $env:GOOS = $null
    $env:GOARCH = $null
    $env:CGO_ENABLED = '0'
    go build -trimpath -o $SiqBinary ./cmd/agentshield
    if ($LASTEXITCODE -ne 0) { throw 'Windows 本机 Go 构建失败' }
} finally {
    $env:GOOS = $SiqPreviousGoos
    $env:GOARCH = $SiqPreviousGoarch
    $env:CGO_ENABLED = $SiqPreviousCgo
    Pop-Location
}
Get-FileHash -Algorithm SHA256 $SiqBinary
```

执行前核对 GOHOSTOS=windows；WSL 中编译/运行另起批次。Windows ARM 的本机构建不因此算 amd64。记录 Go/Node 实际版本与构建输出，受测源码包含 embed；若生成资产有变化，固定干净实现候选后再采正式材料。

### 3.3 启动与管理基线

先创建专用、已核对 ACL 的本机私有测试根。SIQ 状态与智能体 profile 分开；不要用日常默认状态。当前已有启动工具入口为：

```powershell
# $SiqState 必须由 P00 确定为独立、私有的测试状态目录。
# 47612 仅为候选端口，先确认空闲；发现其他服务不得强杀。
$env:SIQ_AGENT_SECURITY_STATE_DIR = $SiqState
python scripts/personal-experience/start-local.py --binary $SiqBinary --state-dir $SiqState --port 47612 --open
if ($LASTEXITCODE -ne 0) { throw '启动/健康检查未通过，保存脱敏诊断' }
& $SiqBinary status --port 47612
& $SiqBinary state-status
```

`$SiqState` 在本节故意不预填：执行者需用自己已确认的隔离目录赋值，未赋值不得运行。保留并在批次后恢复原进程环境变量。配对通过实际 UI 或 `pair --port 47612` 完成，配对码/token 不进终端录制、截图、日志或提交。

验收 A01：正确实例管理页、刷新/新标签页恢复、退出管理失效、服务重启后重新配对、错误端口/错误实例/认证过期能诊断。开发 `dev:local` 的 `/` 和 `/demo` 与 Go embed 深链分别验证；开发服务可用不能替代交付二进制 UI。记录达到首次可用所需步骤/时间，不编造“五分钟完成”。

## 4. Win-P02：三个宿主逐一接入和真实执行

每个宿主各写一行环境、一份结果。使用共享 A02–A08 的相同定义，但不要复制同一内容证据到不同组合。

### OpenClaw

- 先查实际版本、插件加载配置和原生工具回调；读取适配器 README、`index.ts`、安装实现及已有测试。历史测过的 Linux 版本不自动等于本机支持版本。
- 通过已有 `OPENCLAW_STATE_DIR` 隔离能力选择专用实例，核对实际生效路径；不要给用户配置注入未支持的 `security.installPolicy`。
- 从管理 UI 预览、权限确认、安装、自检，到真实宿主调用。检查插件注册不破坏其他 entry、allow/deny 配置；重启后仍加载正确实例，卸载仅移除归属对象。
- hold 必须由宿主可信上下文提供实际执行前复验能力；不能在用户配置补 `approvalExecutionRecheckVersion` 伪造宿主实现。缺能力保持阻断并记录，不只延长超时。
- WSL2 内的 OpenClaw 单独记录进程与 SIQ 所在边界，不把 Windows UI 控制 WSL 当原生。

### Hermes

- 查本机安装方式、实际 CLI/运行时入口、profile 与钩子能力；不要假设 Windows 原生可直接运行所有 Linux Python/shell 探针。
- 复用 `internal/adapterinstall/hermes_native.go`、Hermes 适配器与 managed-instance 流程；先读现有脚本确定支持参数，不照抄 Linux 的固定路径。
- 核对路径含空格/中文、配置备份与还原、子进程参数传递、会话身份、真正的 pre/post 调用以及单次结果关联。
- 宿主等待超时、批准后重试/继续、撤销与最终参数复验单列；普通执行通过不等于审批闭环。不得只向模型提示“已批准”来恢复任务。
- 若实际只能 WSL2 运行，原生行保持明确缺口；提出受控启动/桥接设计时先协调安全协议，不把 loopback 改成对外监听。

### WorkBuddy

- 独立调查其实际 Windows 桌面版、配置、扩展/钩子/Skill 安装方式并在 GUI 观察。资料、版本和出处进入报告；未知项保留 unverified。
- `connectors/workbuddy` 的发现结果可用来定位资产，不能证明执行前阻断或安装拦截。CodeBuddy 的配置和测试不作为替代。
- 验证“界面发起 → 实际工具调用 → SIQ 决策 → 执行/拒绝 → 结果关联”。没有受支持前置入口时记录 host_capability_missing，提出保留原界面的受控启动方案；只有完整执行链可验证才提升能力。
- 不通过修改闭源程序、伪造能力开关或不受支持字段制造表面集成。需要新的适配器时先有接口/能力设计，适配器只做映射，不持有私钥或裁决逻辑。

**P02 退出：** 每个已测组合至少 A02–A05 有真实结果和无副作用负向；A06/A07/A08 缺能力的项继续待办。清晰区分发现成功、钩子加载、受保护调用和可信 Skill 归属，不能一起标绿。

## 5. Win-P03：系统生命周期、文件安全与 N01 原生回归

### 5.1 使用既有命令验证 Task Scheduler

这些是入口清单，不是可以无条件整段执行的脚本。每步先检查当前实例、签名配置和状态目录；读 `main.go` 及对应命令实现确认参数。未知参数应拒绝，不编造 `--force`。

| 顺序/用途 | 已有命令 | 核验 |
| --- | --- | --- |
| 配置导出/准备 | `task-xml`、`task-prepare` | 导出不注册；准备的 SID/路径/实例/签名一致 |
| 注册前后检查 | `task-presence`、`task-register --confirm-register`、`task-query` | 排他创建本用户任务；同名未知对象不覆盖；读回实际系统配置 |
| 运行 | `task-start --confirm-start`、`task-runtime` | 实际进程与正确实例健康匹配，非仅 LastTaskResult=0 |
| 停止 | `task-stop --confirm-stop` | 本实例优雅停止、状态空闲、Writer 可获得；不杀未知 PID |
| 注销 | `task-unregister --confirm-unregister` | 停止且归属复验后删除精确任务，读回不存在，本地历史保留 |
| 组合流程 | `setup_windows_task.go` 及当前 setup CLI | 先核实已有选项，再验证组合与分步结果一致，不能仅看函数返回值 |

补普通用户、重复调用、占用端口、缺失二进制、外来同名任务、XML/源配置篡改、SID 不符、Task Scheduler 不可用、关闭终端、注销/重新登录、重启和休眠唤醒。注册成功不等于已设置登录触发器；核对实际触发条件。需要新增自启/更新链时沿现有规格扩展并附独立确认，不用直接修改系统任务绕过签名。

### 5.2 Windows 特有文件边界

对每个路径用例定义“明确支持且正确执行”或“明确拒绝且无副作用”，两者均可成为边界证据，不能悄悄截断/重写为另一个路径：

- 盘符大小写、不同卷、空格/中文、尾空格/点、保留设备名、ADS、长路径；UNC/网络盘未受支持就显式拒绝，不映射成本地可信目录。
- junction、目录/文件 reparse、symlink、硬链接与目标替换；不跟随到测试根外，不把 POSIX `Lstat` 行为未经验证照搬。
- 文件被浏览器/杀毒/另一个实例占用，rename/link 失败、权限拒绝、只读目标；保持完整签名对象，不退回覆盖发布。
- 私钥/备份/计划目录 ACL、继承权限和其他用户可访问性；`Chmod(0600)` 成功不证明 ACL 满足要求。发现缺口先给最小复现，与共享状态负责人设计修复，不能 blanket `icacls` 修改用户目录。

### 5.3 N01 与升级回退

执行共享 A10：严格标记/未来 reader-writer/重复 JSON/错误实例/嵌套屏障拒绝；验证拒绝前后业务文件清单、摘要、可验证权限和 Grant revision 不变。`state-status` 必须读取 JSON 的 compatible/status，**退出 0 不代表兼容**；它按当前实现可以成功输出“不兼容”诊断。

受支持 v1→v2 用真实旧候选创建历史授权并撤销，新候选显式 `state-migrate --confirm`，核验完整备份、已撤销 Grant、业务字节、重复迁移和中断恢复。旧兼容感知二进制在 v2 上拒写。旧程序 Windows 制品不可得时从可追溯受支持源码构建并记录；不存在可复现源码/身份则该子项 blocked，不能复制 Linux ELF 或把新程序改 Version 冒充旧版。

`service-upgrade/service-rollback` 是已有 Linux 服务路径，不能仅换 Windows 参数就登记 Windows 原生升级。沿现有 clientrelease、task 生命周期和签名 manifest v3 实现/验证所需 Windows 组合；旧/新文件被锁、恢复程序缺失、签名/摘要不符、状态不兼容时副作用前拒绝。元数据迁移备份不承担复活旧权限的回滚功能。

## 6. Win-P04：通知、审批、安装更新与追溯闭环

- 通知：与 Luke/GLM 先统一 `internal/notify` 接口与私密信息规则，再实现 Windows 投递。实际桌面收到、点击进入正确 SIQ 待办且不能直接批准；通知含必要计数，不包含命令/token/私有路径。通知被拒、无桌面、重复事件、超时、重启退避均有降级。不要记录通知子进程原始输出或全局放开执行策略。
- 审批：用隔离文件写入/受控接收端测真实次数。拒绝、过期、撤销、换参数/Skill、平台重试或响应丢失不重复执行；pending 单次消费单测不能替代宿主副作用证明。
- 安装/更新：先本地目录与已支持 HTTPS ZIP；GLM 的安全 Git 通过后再接对应候选。自动检查只提示，权限/内容比较与安装确认仍分开；文件占用/并发移除/上游前进/部分切换故障均不破坏旧记录和用户新增文件。
- 原生安装入口：只有真实安装前可拦截才标 pass；事后发现写清楚。可信 Skill 归属没有受信链就保留 unknown。
- 活动/隐私：真实任务、会话、授权、决策与结果关联；原文默认关闭。测试授权开启/撤销、TTL、损坏密文、导出脱敏、休眠/时钟变化后的过期。结果未知不能标任务成功。
- UX：Windows 浏览器缩放、键盘焦点、中文长路径、加载/空/失败/重试、管理会话刷新恢复；测量步骤和时间后优化，不更换整套 UI。共有 UI 改动与 GLM 协调。

核心未交付时先完善测试方案、最小复现和 Windows OS 部分；不能为通过本机测试绕过安全 Git、Authority、状态屏障或回执关联规则。

## 7. Win-P05：验收、证据工具与提交

### 7.1 必须完成的验证

按实际改动执行，记录每条退出码：

- `apps/agentshield`：`gofmt -l .` 应无输出，`go vet ./...`、`go test ./...`；状态/授权/并发改动跑相关 race。CGO/编译器缺失时只记未运行，不能关闭 race 失败。
- 保留项目四目标交叉构建，并另记录 Windows 本机原生运行。使用不同输出文件，恢复 GOOS/GOARCH/CGO 环境，不让交叉编译设置污染原生测试。
- `apps/web`：`npm.cmd test`、企业构建和本地构建；若修复 npm scripts，再在默认 Windows shell 和 Linux CI 验证。Go embed 与候选必须一致。
- Python/合同：`apps/control-api` 用 `uv sync --dev --locked`，`uv run pytest app/tests/test_schema_contracts.py`；改共享服务再按完整要求扩展。原生 venv 在 Windows 的 Scripts 路径与 Linux bin 不同，优先 `uv run`，不复制 Linux 命令路径。
- 适配器组件与真实宿主 A02–A08、Windows A09/A10/A12 按本批范围运行。mock/system query 输出一致不算真实用户级生命周期通过。

### 7.2 生成和验证 18 行材料

在干净的正式候选上采集，不提交私有状态。下面 `$SiqEvidenceRoot` 是本机已创建的稳定私有证据目录，里面只放脱敏可引用文件；第一次生成后再根据真实结果填写 manifest。

```powershell
$SiqCandidate = (git rev-parse HEAD).Trim()
$SiqMatrix = Join-Path $SiqEvidenceRoot 'matrix.json'
python scripts/personal-experience/platform_acceptance.py init --candidate $SiqCandidate --out $SiqMatrix
if ($LASTEXITCODE -ne 0) { throw '模板生成失败，不能覆盖已有材料' }
# 完成实际测试并按合同填写后，先检查结构/摘要：
python scripts/personal-experience/platform_acceptance.py verify $SiqMatrix --evidence-root $SiqEvidenceRoot --candidate $SiqCandidate --out (Join-Path $SiqEvidenceRoot 'structure-report.json')
if ($LASTEXITCODE -ne 0) { throw '材料结构/身份/摘要不符合合同' }
python scripts/personal-experience/platform_acceptance.py verify $SiqMatrix --evidence-root $SiqEvidenceRoot --candidate $SiqCandidate --require-native --out (Join-Path $SiqEvidenceRoot 'native-report.json')
$SiqNativeExit = $LASTEXITCODE
if ($SiqNativeExit -ne 0 -and $SiqNativeExit -ne 3) { throw '原生材料校验输入无效' }
# 3 = 仍有原生覆盖缺口；如实登记，不删其他 OS 行，不把 3 改成全部通过。
```

尚未测试 macOS/Linux 行保留 not_run；Windows 原生/WSL2 分开填，WorkBuddy 要 native_desktop。只把脱敏后的材料复制到 `docs/evidence/personal-experience/windows-sunbo/<batch>-<timestamp>/`。按共享规则保存 report/environment/verification、命令和摘要，新增 runtime API Schema 另行先设计。

### 7.3 PR 与完成定义

第一批 PR 建议交付 P00 环境报告、P01 实际基线及一个能定位的 Windows 构建/启动适配问题修复（若确有问题），不要为了“有代码”制造无关改动。后续按宿主/生命周期/通知/迁移拆小批。PR 附原失败、最终行为、受测 SHA、实际架构/方式、负向与清理证据、公共依赖以及未运行项。

你负责实测和修复并推送自己的分支、创建 PR；维护者负责审阅合并，不修改 main、绕过门禁或自行发布。功能 PR 可以带明确待验范围，但“Windows 完成”只有所要求组合/旅程全部通过且独立复核后成立；缺原生宿主、WSL 外边界或另一架构不得隐去。

## 8. 直接交给 Windows Codex 的执行指令

> 你是 sunbo 的 Windows 平台开发助手。严格基于本仓库最新上游 main，读取共享协作规则、本任务书、AGENTS、N01 与平台材料规格，建立独立分支。先完成 Win-P00 环境/三个真实宿主盘点和 Win-P01 原生构建、启动、管理验证；设备信息未知由本机核实，不预设版本或架构。随后逐批实测 OpenClaw、Hermes、WorkBuddy、修复 Windows 适配及 Task Scheduler/路径/ACL/通知/升级恢复问题，通过 PR 交付。复用已有核心，与 GLM 协调公共文件和协议，不复制安全引擎。原生与 WSL2 分开、CodeBuddy 不代替 WorkBuddy；实际拒绝用无副作用证据证明，审批重试验证执行次数，未来状态拒写不得绕过。只操作明确隔离的测试实例，保留日常配置；不关闭系统防护、不默认提权或付费调用。先固定干净代码候选再产正式 18 行材料，缺口如实保留；遇到外部阻塞继续独立本机工作。每批落盘代码、测试、脱敏证据并按共享模板报告，再推送分支/创建 PR，等待维护者审阅；不得擅自合并、发布或修改治理。
