# Windows Task Scheduler 干净候选原生验收

受测源码为 `ebc472f2e46aa7de837afe9d6a0ed422eef51cd0`，原生 Windows/amd64 二进制 SHA-256 为 `4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74`。各条 CLI 证据均在调用前检查 HEAD 与干净工作区，并绑定实际二进制摘要；父任务的构建证据另包含 Go 内嵌 `vcs.revision` 与 `vcs.modified=false`。

本轮使用独立新状态 `<NATIVE_TEST_ROOT>\states\lifecycle-candidate-ebc472f2-中文 空格`，端口 47623；程序路径本身含空格。测试根位于仓库忽略目录，ACL 禁止继承，仅当前用户、SYSTEM、Administrators 拥有 FullControl。所有系统动作仅针对本轮创建的精确任务；状态、密钥和历史未复制到公开证据。

## 原生行为与证据

| 检查 | 结果与证据文件 |
| --- | --- |
| 初始化、注册、完整配置读回与重复注册 | `candidate-init.json`、`candidate-register.json`、`candidate-register-repeat.json`；全部 exit 0，中文路径保持正确 |
| 普通按需启动 | `candidate-normal-reregister.json` → `candidate-normal-ready.json` → `candidate-normal-start.json` → `candidate-normal-running.json`；先确认 ready/0，再启动并确认目录健康与 running/1 |
| 命令退出后服务继续运行 | `candidate-runtime-running.json` 是发起启动的独立命令结束后的另一次调用；`candidate-process-identity.json` 以 CIM 和 TCP 读回确认唯一进程的程序路径、状态参数以及同 PID 的目标端口监听 |
| 重复启动 | `candidate-start-repeat.json` exit 0，复用现有健康实例 |
| 运行时拒绝注销 | `candidate-unregister-running-denied.json` exit 1，主 Writer 锁由服务持有；随后正常停止成功，任务没有被提前删除 |
| 正常停止、注销、重复注销 | 首轮 `candidate-stop.json`、`candidate-runtime-stopped.json`、`candidate-unregister.json`、`candidate-unregister-repeat.json`、`candidate-final-absence.json`；补轮 `candidate-normal-stop.json`、`candidate-normal-stopped.json`、`candidate-normal-unregister.json`、`candidate-normal-final-absence.json`。均在停止后 ready/0/LastTaskResult=0，最终 absent |
| 同名外来系统配置 | `candidate-foreign-*-denied.json` 中 register/start/unregister 均 exit 1，理由均为完整系统配置不匹配；`candidate-foreign-ownership-observation.json` 证明系统定义和全部状态文件 hash/attributes 不变、fixture 从未运行，且精确清理成功 |
| 占用端口 | `candidate-port-occupied-start.json` exit 1，未报告目录健康；`candidate-port-occupied-observation.json` 证明测试创建的占用者仍绑定，未停止未知进程 |

上述退出码 1 均是预期负向，并非被抹去的失败。占用端口的边界仅限“占用期间不报告就绪且保留占用者”。释放监听后，后续 `candidate-after-occupied-runtime.json` 已读到 running/1；可能是延迟启动跨过了 CLI 健康超时，不能据此声称服务因 bind 冲突退出。另行注册并从 ready/0 启动的 `candidate-normal-*` 证据用于证明普通路径。

同名外来配置是本轮在确认任务缺席后创建的独立 fixture，只给合法 Definition 增加未签名 Description；没有替换或改写任何既有任务。清理前完整复验任务名、Definition、ready/0 和从未运行状态，产品的三次归属拒绝均没有改变 fixture 或本地状态。

## 代码回归与原失败

候选关键 Go 组通过：`go test -p 1 -count=1 -timeout=10m ./cmd/agentshield -run '^Test(Windows|StateStatusReportsAncestorBarrier)'`，exit 0，包执行 185.584 秒，总持续 282.193 秒，stderr 为空。Go 1.27.1 windows/amd64，CGO_ENABLED=0，TEMP/TMP 使用非 AppData 私有目录；开始与结束时 HEAD 均匹配候选且工作区干净。

完整参数、版本、原始退出码与首尾源码状态见 `candidate-windows-go-tests.json`，脱敏输出见 `candidate-windows-go-tests.log`。测试范围包括原生 COM XML 输入/读回、精确缺席判定、实际 stderr 拒绝、XML 归属负向与旧签名准备状态不改写，以及 state-status 祖先路径屏障。

开发阶段的原始失败独立保留在开发报告与 `baseline-*`、`fixed-*`、`signed-*` 证据中，不计作干净候选结果：原生 PowerShell 的 FileNotFoundException 包装；progress CLIXML；BSTR 与 UTF-8 声明冲突；OEM/UTF-16 声明与中文路径；Task Scheduler 完整默认项与统一调度引擎回写；以及 MSIX AppData 短路径不可供包外任务直接定位的问题。

本机已通过 `GetFinalPathNameByHandleW` 确认最初 AppData 短路径的真实落盘位于 Codex MSIX LocalCache。该次任务启动返回 `0x80070002`，保留为环境边界失败。改用非 AppData 私有根、全新状态后才获得本报告的原生成功链；没有搬迁或改写旧状态来掩盖失败。

生成器现把 `UseUnifiedSchedulingEngine=true` 明确纳入签名配置，归属校验仍精确比较该字段。旧准备记录不自动迁移、不重签，新工具拒绝不匹配配置并保持原文件。旧签名和匹配旧工具必须保留，实际迁移应由维护者另行明确设计。

## 范围与清理

这证明当前 Windows 用户会话中的按需 Task Scheduler 生命周期、目录身份和归属拒绝。没有实测用户注销/重新登录、重启、休眠恢复、桌面通知或正式升级/回退；不把命令进程退出扩大解释为人工关闭终端 GUI。任务未配置登录触发器，不能声称登录自启。

这些系统证据也不证明 OpenClaw、Hermes 或 WorkBuddy 的执行前拒绝能力，不能据此将 Windows A12 或三宿主整体标为通过。最终精确任务、进程和数据保留状态见 `candidate-cleanup-observation.json`。
