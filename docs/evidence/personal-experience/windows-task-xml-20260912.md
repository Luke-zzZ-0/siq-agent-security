# M73：Windows 用户任务配置导出

日期：2026-09-12。候选：codex/personal-macos-stop-recovery 基于 274ed97 的 M68–M73 本地增量。规格 §3.11.35；task-xml，无系统注册写入。

任务 XML 绑定当前用户 SID、实例 ID、当前程序和显式状态目录。InteractiveToken/LeastPrivilege、没有触发器，允许按需启动、拒绝强制终止、不设执行时限。命令直接运行二进制，不经 shell，不使用可能被任务引擎展开的百分号路径。SID 来自标准库当前用户，不接受任意账户参数。

设计依据为 [Microsoft Task Scheduler Schema](https://learn.microsoft.com/en-us/windows/win32/taskschd/task-scheduler-schema)、[AllowHardTerminate](https://learn.microsoft.com/en-us/windows/win32/taskschd/tasksettings-allowhardterminate) 和 [LogonType](https://learn.microsoft.com/en-us/windows/win32/taskschd/principal-logontype)。本批没有运行 Windows 任务引擎或其 XSD 验证器；格式依据官方 XSD，实际验证为 Go/Python XML 解析及字段关系检查。

## 验证

- Go vet、全量、CLI race、160 项 Python 合同/样例与 Ruff 通过。
- Go 输出与共用 windows-task.sample.xml 逐字节一致；Go/Python 分别解码 XML，核对用户身份、动作上下文、设置、程序和带空格/中文/ampersand 的参数转义，无触发器。
- UNC/设备/相对路径、遍历、空分段、末尾点/空格、保留字符、环境展开、控制字符、非法 UTF-8、长度超限拒绝；260 单元边界通过。服务账户、非法 SID 数值/结构及非法实例摘要拒绝。
- 四目标构建通过，gofmt、git diff --check 通过。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | ba88edc7bf506a2026914d064203e4eea3901a6bf6e04e5f92578842c46025d3 |
| linux/amd64 | 7a93ef5f151ffcea8e0461a55efd3280c381be9ec27e0dfbe7ebb952cfb9cd48 |
| darwin/arm64 | e62d5d20b5be739ad50007f9e38627727ec51305d2b56a0c9a80c248bb2f648f |
| windows/amd64 | 6d12c7eb464688dc439525488966f146199aaec83fdd932cf0d3b1cf633e1b55 |

配置导出不算 Windows 后台支持完成；签名归属、系统读回/注册、启动与正常退出、实机验收待推进。PR #31 内容保持不变，本批不提交或推送。
