# Windows 实机环境记录

日期：2026-09-14，Asia/Shanghai。范围：Win-P00 环境盘点，以及本批 Windows 原生构建、管理会话、状态诊断和 Hermes 组件验证。本文只记录普通系统与软件身份、测试方法和隔离边界；不包含账号、个人用户名、SID、机器标识、私有绝对路径、凭据或启动日志。

本批为部分平台交付。OpenClaw、Hermes、WorkBuddy 的完整原生用户旅程尚未验收，N09 不关闭。Task Scheduler 的最终执行与清理结果以同批 `report.md` 为准。

## 1. 受测代码与制品身份

| 字段 | 实际值 |
| --- | --- |
| 仓库 | `maoyadongsh/siq-agent-security` |
| 分支 | `codex/windows-sunbo-validation` |
| 开发复现 main 基线 | `aac1154bd0fe25d38f451716f1b1edd385f6358a` |
| 正式实现 `candidate_sha` | `ebc472f2e46aa7de837afe9d6a0ed422eef51cd0` |
| Hermes 正式复验工作树 | 执行前后均为该候选，`source_dirty=false` |
| Windows 原生二进制 SHA256 | `4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74`；Hermes 复验前后摘要不变 |
| Go 二进制元数据 | 实读 `vcs.revision` 与上述候选匹配，`vcs.modified=false` |
| PR | [PR #40：Windows 原生构建、计划任务与状态诊断首批适配](https://github.com/maoyadongsh/siq-agent-security/pull/40) |
| 材料身份 | 实现候选与后续证据提交分开；PR 后继证据提交不改变原始测试的候选身份 |

原开发失败材料保留原 SHA、原摘要和 `source_dirty=true`，没有改写为正式候选通过结果。

## 2. Windows、工具链与浏览器

| 项目 | 实际值与来源 |
| --- | --- |
| Windows | Windows 11 专业工作站版 25H2；版本 `10.0.26200`；完整 build `26200.8875`，来自本机 OS 与系统版本信息 |
| OS / PowerShell 进程架构 | x64 / x64，来自系统运行时 API |
| PowerShell | 7.6.5 |
| Git | 2.51.0.windows.1 |
| Go | Go 1.27.1，Windows amd64 原生工具链 |
| Go 来源 | 官方 Windows amd64 ZIP；安装包 SHA256 `a3911b5e0e1b1053f25ed0675f4c1c6aad1e2bfcf253df2b9be4caabd2edd95d` |
| Go 当前探测环境 | `GOHOSTOS=windows`、`GOHOSTARCH=amd64`、`GOOS=windows`、`GOARCH=amd64`；当前探测进程的 `CGO_ENABLED=1`，不能据此推导具体制品或 race 已运行 |
| 正式 Windows 制品构建设置 | 从该二进制实读 `CGO_ENABLED=0`、`GOOS=windows`、`GOARCH=amd64`、`GOAMD64=v1`；与上行当前探测进程的默认设置分开记录 |
| Node.js / npm | v22.20.0 / 11.18.0；Windows 原生命令使用 `npm.cmd` |
| 系统 Python / uv | Python 3.13.7 / uv 0.12.13；Hermes 自带解释器另列 |
| 管理会话测试 | Playwright 1.56.0，headless Chromium，browser revision `1194` |
| 测试 Chromium 实际版本 | `141.0.7390.37`；对已安装的 `headless_shell.exe` 在隔离环境中只运行 `--version`，退出 0；同 revision 的 Chromium 程序文件元数据亦为该版本 |
| 系统 Microsoft Edge | 文件元数据 `153.0.4234.32`；仅盘点，不作为本批 browser smoke 的受测浏览器 |
| 系统 Google Chrome | 文件元数据 `152.0.7977.83`；仅盘点，不作为本批 browser smoke 的受测浏览器 |

浏览器版本查询没有打开用户浏览器窗口或使用其日常 profile。浏览器 smoke 为独立 headless 实例，不能代替 WorkBuddy 桌面或三个宿主真实工具执行证据。

受测 SIQ、Hermes 自带 Python 和测试 headless Chromium 的 PE Machine 均实读为 `0x8664`（amd64）。Hermes 原始 diagnostics 中 `platform.machine()` 返回空字符串，该原字段原样保留；架构由独立 PE 头和 OS API 证据补充，不将空值静默重写。

## 3. 文件系统、隔离与跨进程路径

| 项目 | 已核对事实 |
| --- | --- |
| 文件系统 | 工作区与本批测试根位于本地 NTFS；同步盘、网络盘及 WSL 共享目录没有作为已验收文件系统 |
| 私有父根 | ACL 继承保护开启（Protected=true），3 条显式 FullControl ACE，分别属于当前用户、SYSTEM、Administrators；无宽泛 ACE；根本身不是 reparse point |
| SIQ 状态 / Hermes profile / 工具工作区 | 各自独立，使用新的私有测试子目录，不使用日常默认状态 |
| 端口 | SIQ 仅在本机 loopback 使用探针选择的测试端口；冲突和错误端口验证见同批报告 |
| 模型 | Hermes diagnostics 未调用模型，付费模型调用为 0 |
| 敏感材料 | 私有状态、密钥、完整配置、原始启动日志与失败现场留在本机；公开材料只含脱敏结果和摘要 |
| Hermes 复验清理 | 自建活动进程数为 0；私有状态留存，未删除 Writer 锁；失联测试使用终止本探针自建 SIQ 子进程的方法，不声明完成优雅停止/重启验收 |

开发执行环境存在 MSIX AppData 路径虚拟化。通过 Win32 `GetFinalPathNameByHandle` 已确认：开发进程可访问的短 AppData 路径可能不是 Task Scheduler 外部进程可打开的实体路径。旧测试任务已按归属注销；正式测试改用工作区内新的私有测试根。实际系统任务的注册、运行与注销结果引用同批报告，不由目录可写性或程序存在推导。

本批局部 ACL 检查不等于跨用户访问、备份 ACL 保持或完整 OS 沙箱验收。junction/reparse、硬链接、目标替换、UNC/ADS、保留设备名、尾点/空格、不同卷、文件占用及每类迁移中断的完整 Windows 边界仍未全面验收。

## 4. 三宿主当前版本与实测边界

| 宿主 | 版本与来源 | 本机运行方式 | 本批实际强度与未验收项 |
| --- | --- | --- | --- |
| Hermes | 本次重新读取运行源码版本 `0.21.2`、release 标记 `2026.9.11`；自带 Python `3.11.16` | 已安装 Windows 原生 CLI、Python 与桌面；本次用其原生 Python 运行宿主 Hook 组件 | 独立 `HERMES_HOME` profile 中加载插件并调用宿主 `pre_tool_call` 分发器，服务失联返回 block。method 为 `component_fixture`；完整公共 CLI/工具 dispatcher、desktop、审批继续和可信 Skill 归属未验收 |
| OpenClaw | 本次读取 Tray 文件版本 `2026.9.3.0`；WSL runtime 包 `2026.9.4`，Node `v24.19.0` | Windows Tray 与 WSL2 gateway；已存在 gateway 在读元数据时运行 | 尚未确认可用的 Windows 原生 CLI/runtime 入口。未接入实际 runtime，隔离配置、插件加载、前后调用与 Windows↔WSL2 网络/文件边界均未验收 |
| WorkBuddy | 本次程序文件元数据 `5.5.6`，ProductVersion `5.5.6.0`；同日真实桌面已打开观察 | Windows 原生桌面 | 官方有 Hook 插件类别；执行前拒绝合同、调用/会话身份、最终复验及隔离 profile/项目配置入口仍 `unverified`。完整桌面→工具→SIQ→执行/拒绝→结果关联未验收 |

Hermes 同日 P00 盘点的桌面 package 为 0.17.2；其桌面可执行文件的 40.10.2 是 Electron 元数据，不作为 Hermes 运行时产品版本。安装源码预先存在工作树差异，本批未修改；正式 diagnostics 对受测 CLI 与源码文件记录摘要，前后相同。

OpenClaw 同日安装登记版本为 2026.7.1-2，与实际 Tray 文件元数据不同，差异保留；runtime 版本独立采用本次重新读取的 WSL 安装包值。Tray 存在不代表 Windows 原生 OpenClaw runtime 已验证。

WorkBuddy 实际配置布局与仓库 Connector 约定输入不完全吻合。未创建虚构 profile 使发现通过；CodeBuddy 配置、CLI 文档与测试不替代 WorkBuddy。现有调查不足以断言 `host_capability_missing`，只能保留未证实状态。

## 5. WSL2 单独登记

| 项目 | 事实 / 边界 |
| --- | --- |
| 已观察宿主 | 已存在的 OpenClaw gateway 在 WSL2 中运行；本次只读包版本与 Node 版本成功 |
| Guest | Ubuntu 24.04.4 LTS，同日 P00 元数据 |
| 内核 / 架构 | `6.6.87.2-microsoft-standard-WSL2` / x86_64，同日 P00 元数据 |
| SIQ 所在系统 | 本批受测 SIQ 是 Windows 原生；没有开展 WSL2 SIQ 集成测试 |
| 网络边界 | Windows SIQ↔WSL2 OpenClaw 的 loopback 可达性、代理影响、实例身份、失联与恢复未验收 |
| 文件边界 | Windows 盘符、WSL Linux 路径、共享目录与最终执行资源绑定未验收 |

Windows 浏览器能打开页面或 Tray 能显示状态，都不能把 WSL2 runtime 计入 Windows native 行。其他 OS/架构的未测组合保留在完整 18 行矩阵中。

## 6. 本环境已完成的正式验证范围

以下为本批负责人的正式结果汇总，用于说明环境实际可运行的验证范围；完整命令、退出码、计数与证据摘要仍以同批 `report.md` 和验证 JSON 为准。

| 验证 | 本批结果 |
| --- | --- |
| Go 格式 / vet | `gofmt -l` 无输出；`go vet` 退出 0 |
| 四目标构建 | linux amd64、linux arm64、darwin arm64、windows amd64 均退出 0；交叉构建不算另一 OS 的原生实测 |
| 管理浏览器会话 | 11 项为 true；Playwright 1.56.0 / headless Chromium 141.0.7390.37 |
| Windows 状态负向 smoke | 6 项通过；只覆盖声明的拒绝诊断与零业务字节变化范围 |
| launcher | 13 项通过、1 项条件跳过；跳过原因引用同批报告，未把条件跳过填为通过 |
| Hermes diagnostics | 退出 1，`observed_failure`；Windows 路径合同 fail，原生 Hook 组件失联 block pass；两次原生 Python 子进程均退出 0，受测宿主源码摘要不变 |
| Task Scheduler | 正式系统任务生命周期与清理结果由同批报告独立登记；不从组件结果推导 |

本环境记录没有声明全量 Go、race 或 Windows Control API 已通过；其实际结果/平台限制由同批报告单独列出。登录、注销后恢复、重启、休眠唤醒、升级回退、通知投递、完整审批单次副作用、最终参数复验和可信 Skill 归属均未完成平台验收。

Hermes 的 Windows 文件资源路径失败继续由 [Issue #39](https://github.com/maoyadongsh/siq-agent-security/issues/39) 跟踪。POSIX 绝对路径返回 201，而两种 Windows 盘符绝对路径均返回 400 / `intent_invalid_resource_constraint`；相对路径和盘符相对路径负例亦返回 400。没有用 regex、伪路径或放宽 Authority 绕过，完整 A04/A05 native_cli/native_desktop 不因此标绿。

## 7. 证据解释与来源

同批 Hermes 原始脱敏结果见 [diagnostics](evidence/hermes-native-diagnostics.json)；候选、执行前后 Git 状态、真实退出码和文件摘要见 [provenance](evidence/hermes-provenance.json)；当前工具/宿主版本见 [环境 JSON](evidence/environment.json)；PE 架构与 headless Chromium 实读版本见 [身份补充](evidence/runtime-identity-observations.json)。四份原始脱敏 JSON 均保留原字节。

原 diagnostics 脚本硬编码 `source_state=development-exploration` 且本轮架构字段为空，因此保留其原文。外层 provenance 用现场观察绑定新的干净候选，独立身份记录补充架构事实；没有改写原报告，也没有把 Hook 组件提升为完整原生执行证据。

官方资料核对日期为 2026-09-14。文档只用于定位支持入口，不能代替本机行为：

- [OpenClaw Windows](https://docs.openclaw.ai/platforms/windows)：Windows 与 WSL2 的运行方式。
- [Hermes Windows Native](https://hermes-agent.nousresearch.com/docs/user-guide/windows-native)：Windows 原生与独立数据目录。
- [Hermes Plugins](https://hermes-agent.nousresearch.com/docs/user-guide/features/plugins) 与 [Tools Runtime](https://hermes-agent.nousresearch.com/docs/developer-guide/tools-runtime)：插件和工具 Hook 入口。
- [WorkBuddy 插件系统](https://www.codebuddy.cn/docs/workbuddy/Plugins)：存在 Hook 类型；本批所需的执行前 veto 与隔离 profile 完整合同仍未证实。
- [Microsoft Task Scheduler Settings](https://learn.microsoft.com/en-us/windows/win32/taskschd/taskschedulerschema-settingstype-complextype)：任务设置定义。实际任务仍按签名与严格读回比较，未知字段、权限变化与旧签名漂移不放宽。

P00 已形成可信的设备/三宿主环境基础，并在正式候选完成了上述部分复验。平台整体验收、独立同 OS 完整重跑与全部用户旅程尚未完成。
