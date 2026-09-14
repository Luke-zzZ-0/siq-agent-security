# Windows 首批适配与实机验证

本批完成 Win-P00/P01 的环境、构建、启动和管理验证，并修复、实测部分 Win-P03 生命周期和状态边界。**Windows 平台总体验收仍未完成，Win-P02–P05 与 N09 不关闭。**

| 身份 | 实际值 |
| --- | --- |
| 上游基线 | `aac1154bd0fe25d38f451716f1b1edd385f6358a` |
| 实现候选 | `ebc472f2e46aa7de837afe9d6a0ed422eef51cd0` |
| 分支 / 审阅 | `codex/windows-sunbo-validation`；[草稿 PR #40](https://github.com/maoyadongsh/siq-agent-security/pull/40) |
| 正式源码状态 | `source_dirty=false`；Go 内嵌 `vcs.revision` 匹配，`vcs.modified=false` |
| Windows amd64 二进制 SHA256 | `4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74` |
| 发行身份 | 本机自建 `0.0.0-dev`，没有发布签名、安装包或支持承诺 |
| 证据提交 / 最终 PR head | 证据单独提交；准确提交身份及最终 CI 在 PR 描述中记录。本批证据引用上述实现候选，不重标历史 SHA |

环境、命令和文件摘要分别见 [environment.md](environment.md)、[verification.json](verification.json)。原始状态、密钥、配对输出与有私人路径的日志仅本机保留；归档只包含脱敏结果。开发失败明确标记 dirty，与正式复验分开。

## 修复及原生行为

| 实际问题 | 修复与证据边界 |
| --- | --- |
| Windows shell 无法识别 npm scripts 中的 POSIX 环境赋值 | 改为无新增依赖的 Node 入口；企业和本地构建、开发入口均实际运行。214 个 embed 文件与候选 Git blob 逐字节相同 |
| Git autocrlf 改变签名样本、规则、Skill、内嵌资源字节；Go 格式检查也受影响 | 固定检出规则，只恢复能与原 Git blob 匹配的 LF 字节。未修改规则或 Skill 的签名来适配错误换行；任务 XML 的签名变化只对应下述实际设置变化 |
| Windows 空闲回环连接约两秒才返回明确拒绝，旧一秒预算误判；locale 解码也失败 | Windows 探针最多五秒，只接受明确连接拒绝；超时、权限错误和未知服务仍拒绝。就绪等待保持默认十五秒，可显式 `--timeout 60`。JSON/诊断明确按 UTF-8 处理 |
| 浏览器探针未 init，或用 Windows 强制终止冒充正常停止 | 新状态先初始化，正常路径通过 `stop --confirm-stop` 排空；失败清理只涉及自建子进程，保留失败，不删 Writer 锁 |
| Task Scheduler 的 COM XML/BSTR 编码、缺席异常和默认设置读回不符合旧假设 | 固定 PowerShell 脚本、JSON stdin、明确 UTF-8；仅抑制 progress，实际 stderr 仍拒绝。缺席只接受定点 GetTask 的精确类型/HRESULT；完整读回使用 `Definition.XmlText` |
| 调度器在本机将引擎设置读回为 true | 将 `UseUnifiedSchedulingEngine=true` 显式纳入签名源并精确比较；只允许原源省略时系统补全空 Triggers 与 DisallowStartOnRemoteAppSession=false。未知、重复、属性、子项、权限提升及非默认值继续拒绝 |
| 有效内层状态使 `state-status` 隐藏未来/损坏祖先标记 | 诊断检查全祖先屏障，输出 compatible=false 与 future/corrupt；保持退出零可输出不兼容诊断的合同，不扩大写入能力 |

旧的已准备任务记录不会被自动重签或迁移。负向测试核对新源被拒后旧签名仍有效，文件清单、字节和 mode 不变；这不等于 Windows ACL 保存认证。旧实例需保留匹配工具与签名源，由维护者设计明确迁移，不能建议删状态恢复。

开发时还发现 Codex MSIX 的 AppData 虚拟化：开发进程可见的短路径，CLI 健康检查超时，随后任务读回 `LastTaskResult=0x80070002`（有符号十进制 -2147024894）；该码是任务结果，不是 CLI 退出码。Win32 句柄最终路径证实该边界。旧测试任务按精确归属注销；随后新建工作区 ignored `.tmp` 下的私有 NTFS 根、复制二进制并 **重新 init 新状态**，没有搬迁带目录身份的旧状态。该根仅当前用户、SYSTEM、Administrators 三条 FullControl ACE，继承保护开启。最终生命周期证据记录该新路径可见性；不将应用隔离路径问题误报为系统服务已运行。

## 正式验证结果

| 检查 | 结果与范围 |
| --- | --- |
| 本机构建与交叉编译 | windows/amd64、linux/amd64、linux/arm64、darwin/arm64 全部退出 0；只有 Windows 二进制在本机执行 |
| Go 格式 / vet | `gofmt -l .` 无输出；`go vet ./...` 退出 0 |
| 完整 Windows Go / 关键组 / race | 全量退出 1，耗时约34分40秒：观察到顶层测试934通过、61失败、48跳过；包级23通过、16失败、6无测试。server与skillinstall两个包达到默认十分钟超时；未到达的断言不计通过。Windows/StateStatus关键组退出0；stateformat/statefs的race退出0（1.495s/3.170s），不代表整个状态/授权引擎已做race验收 |
| Web | 22 个文件、74 项测试通过；企业/本地构建和 dev:local --help 退出 0；6 个 HTTP 入口正确；2 个自启 Node 进程及端口清理；214 个 embed 文件匹配 Git |
| 启动器 | 13 通过、1 跳过，退出 0；跳过的是原有 Unix shebang fixture，三个实际 Windows binary 测试函数通过；本次就绪预算显式 60 秒 |
| 管理浏览器 | 真实 headless Chromium 141.0.7390.37，11 项通过：配对、Cookie 范围、脚本无凭据、刷新/新标签恢复、退出撤销、重启重新配对、连接重试、窄屏布局和页面错误 |
| N01 合成负向 | 未来 reader、未来 writer、重复键、错误实例、错误目录、嵌套祖先六项通过；拒绝前后树清单/摘要一致。没有冒充真实旧→新迁移或断电恢复 |
| Task Scheduler | 真实注册、普通 ready→start→running、重复启动、运行中拒绝注销、stop→ready/0/LastResult0→注销/重复注销成功；外来同名定义下注册/启动/注销三次严格拒绝，系统定义及文件 hash/attributes 不变。占用端口仅证明不报告就绪和占用者保留；不声称服务已因 bind 冲突退出。关键 Go 组退出 0（185.584 秒），详见 evidence/lifecycle/report.md |
| Python 合同 | 原任务书命令退出 4：全局 conftest 导入 Linux-only `fcntl`；UTF-8 下独立 `--noconftest` 校验 206 项通过，退出 0。未声明 Windows Control API 支持 |
| 平台校验器单测 | 39 项运行，37 通过、2 条件跳过，退出 0；校验器测试不等于三 OS 宿主验收 |
| 18 行矩阵 | 全部候选行保留；结构校验退出 0；`--require-native` 退出 3，仍需原生证据。系统生命周期和浏览器结果没有冒充八项宿主检查 |
| 远端 CI | 实现候选对应检查 38 成功、3 条件跳过，包括 Linux Web/API/Go 和新增 Windows build-inputs；最终证据提交后的 CI 另在 PR 记录 |

Windows 默认 GBK 环境直接读取未指定编码的中文 schema fixture，也曾得到 100 失败/106 通过；设置 `PYTHONUTF8=1` 后上述独立合同组通过。新 Windows CI 明确设置该变量。原失败没有被改报为通过。全部计数按各自命令报告，不把子测试、包事件、跨编译和宿主检查混成总分母。

## Hermes 与其他宿主

正式 Hermes diagnostics 在同一干净候选上 **退出 1**。同一 Intent 模板仅替换 filesystem prefix：POSIX 绝对路径返回 201；Windows 反斜杠/正斜杠盘符绝对路径均返回 400 / `intent_invalid_resource_constraint`；普通相对和盘符相对路径负例也返回 400。未执行文件副作用。共享问题见 [Issue #39](https://github.com/maoyadongsh/siq-agent-security/issues/39)，应由共享核心负责人先统一资源规范化、哈希与匹配合同，再由 Windows 复验。

本机 Hermes 0.21.2 的插件加载器与 `pre_tool_call` 分发器在服务失联时返回 block，哨兵不存在，宿主源码摘要前后相同。方法仅为 **component_fixture**：没有完整工具 dispatcher、公共 CLI 或桌面执行链，因此不能据此证明实际工具被宿主阻止。制造失联时只终止此探针自己的 SIQ 子进程，保留私有 Writer 锁，不声称优雅停止或崩溃自动恢复。没有真实模型调用、付费调用或用户凭据。

探针原报告保留硬编码的 `source_state=development-exploration`；新执行的候选/clean 身份由外层 provenance 独立绑定，原 JSON 未改写，方法也未升级。部分 Python 环境返回空 architecture；保留原观察，另通过 PE 头与独立运行核对 amd64，未从空值猜架构。

| 组合 | 本批真实结论 | 后续验收 |
| --- | --- | --- |
| Windows + Hermes native | 已安装原生 CLI/Python/桌面；路径准备失败，Hook 组件失联 block | 等待共享路径合同修复，重跑安装→允许→越权拒绝→审批/最终复验→归属→卸载 |
| Windows + OpenClaw native | Tray 存在；未取得已确认的原生 CLI/runtime 入口 | 不能用 WSL2 runtime 代替原生行 |
| Windows + OpenClaw WSL2 | 实际 runtime 2026.9.4、Ubuntu 24.04.4/WSL2 | 独立 profile、Windows↔WSL2 网络/文件路径与工具完整链未测 |
| Windows + WorkBuddy native desktop | 5.5.6 实际桌面打开；官方列出 Hook 类别 | 前置 veto、最终参数、会话身份、隔离 profile 合同尚未证实；保持 unverified，不能用 CodeBuddy 替代 |
| macOS / Linux / 其他候选行 | 本批未执行相应宿主旅程 | 保留 not_run；交叉构建和 CI 不能替代真实宿主证明 |

## 验收映射、保留现场与下一批

| 要求 | 本批状态 |
| --- | --- |
| P00 环境 | 设备和三个宿主的安装/运行方式盘点完成；发现不等于已接入保护 |
| A01 / P01 启动与身份 | 指定状态/端口启动、管理配对/会话、任务进程与状态/端口身份已实测；所有错误实例和认证失效组合尚未逐项完成 |
| A02–A08 / P02/P04 | 三个真实宿主的发现/接入旅程仍待验；路径准备真实失败及组件失联结果已归档；允许/拒绝真实执行、审批继续、最终参数、Skill 归属、安装拦截/更新尚未验收 |
| A09 通知 | OS 通知投递、点击待办、脱敏和降级未验收 |
| A10 / P03 状态与发行 | 六项合成状态拒绝实测；ACL 备份保持、其他用户访问、文件占用/硬链接、实际旧→新迁移、签名制品兼容预检和中断恢复仍未完成平台验收 |
| A11 活动与隐私 | 主体/版本或 unknown、授权/决策/结果关联、原文默认关闭，以及导出/过期/撤销的隐私边界尚未完整验收 |
| A12 / P05 / N09 | 无登录触发器；未注销登录、重启机器、休眠、Windows 升级/回滚或完成产品卸载总旅程，整体不 accepted |

完整Go残余失败、源码依据和后续主修建议见 [go-gaps.md](go-gaps.md)，分层统计见 [go-final-summary.json](evidence/go-final-summary.json)。需协调的实际能力包括Windows文件资源规范化、可信制品/Connector原生可执行入口，以及崩溃后Writer保守拒绝；跨OS路径、JSON转义、POSIX权限/文件替换和symlink攻击构造另列测试前提问题。两个包级超时仍需同预算隔离测量，不能归咎具体系统组件或通过放宽断言/延长预算宣称解决。

正常管理与生命周期测试使用产品停止路径；浏览器、launcher 和自启 Node 进程已结束。Hermes 失联 fixture 的强制终止单独记录，其 Writer 锁和私有现场保留。生命周期最后通过 COM/CIM/TCP 独立读回：5 个测试任务全部 absent，匹配本子任务程序/状态的进程 0，测试端口监听 0；签名 XML、配置、密钥与历史保留。全量Go与race结束后的CIM核对也确认本批私有根下进程为0（evidence/final-process-observation.json）；未删除任何锁来伪造恢复，未改用户日常宿主配置。

新增代码与合成数据为本批原创贡献，按仓库许可并附 DCO sign-off；未新增第三方运行依赖。代码已接受独立审阅；未声称第二位操作者独立完成全部同 OS 验收。维护者审阅/合并，Windows 负责人继续复验本机，共享状态/资源合同问题由共享核心协调。没有合并、发布、修改治理或改动 V5 冻结材料。

旧→新 Windows 迁移的源码前提与后续命令见 [migration-next-batch.md](migration-next-batch.md)。当前可达历史中未找到可追溯完整源码的兼容 v1 候选；历史基线及仅有的旧二进制摘要不能代替该身份。该项需维护者提供完整历史快照或准确基线与补丁后再验收。

WorkBuddy 的下一批准备见 [安装源码只读调查](workbuddy-next-batch.md)：已定位桌面配置目录入口、插件设置传递链和内置组件的 PreToolUse 拒绝合同。普通退出码 1 不阻断工具；有效 deny 或规定的退出码 2 才能用于后续拒绝验证。该调查未运行桌面工具，不能证明账号隔离、实际插件加载或工具副作用已阻断，矩阵保持 not_run。
