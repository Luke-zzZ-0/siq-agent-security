# M77：Windows 定点任务存在性查询

日期：2026-09-12。规格 §3.11.39；当前后继分支基于 274ed97，本批未提交。命令 `task-presence`。

实现：先核对本地签名准备，固定内嵌 PowerShell 脚本连接当前用户本机 Schedule.Service、打开根目录，再定点 GetTask。只在该 GetTask 调用的 COM 文件不存在异常中返回缺席；连接、目录和其他错误保留失败。系统目录定位 PowerShell，无 profile、无 ExecutionPolicy bypass、非交互，参数通过 stdin JSON，不插入脚本文本。脚本还核对当前 WindowsIdentity SID。

Go 只接受零错误、空 stderr 和精确固定响应。存在须继续通过 schtasks 的完整配置读回；缺席须复验本地签名源。成功输出 present/absent，不写系统任务，不改变当前运行进程。

## 验证

- Go Windows 定向、全量、vet、CLI race 通过（race 5.477 秒）；四目标构建通过。
- 七项临时状态/签名模拟编排：存在、缺席、查询失败、无归属、同名异配置、配置查询失败、缺席查询期间本地源改变。错误不产生状态输出；缺席不调用配置查询，无归属不接触系统查询。
- 内部协议精确响应、stderr、非零错误、空输出、尾随换行、冲突响应负向通过；EncodedCommand 的 UTF-16LE/base64 固定非 ASCII 向量通过。
- 当前宿主未发现 pwsh，也没有 Windows 系统。未运行或用 PowerShell 解析该脚本；Go 编译仅证明嵌入成功，COM 调用/异常包装及系统 PowerShell 兼容均待原生验收。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 610774489d922452aa342052dcd090444d2d0d15b812b3a24fce7ec085c49c58 |
| linux/amd64 | d6473e15665828c3a00cae3710b1359959922b3c3e88a50cc2d8d28432707bb7 |
| darwin/arm64 | 5ae1141f79ebaea18706a3f1c83f4fe779c3748d932c13d8efb89c33dcc2a625 |
| windows/amd64 | a7d638f97b5b6284ebae2dc902ef014bcc6b02d00cf8cd7b4a7a4c75fe03f1d8 |

## 设计依据与后续

[TaskService.Connect](https://learn.microsoft.com/en-us/windows/win32/taskschd/taskservice-connect) 与 [TaskFolder.GetTask](https://learn.microsoft.com/en-us/windows/win32/taskschd/taskfolder-gettask) 支持当前令牌本机连接和指定任务查询；[系统错误码](https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--0-499-)区分文件不存在与拒绝访问。缺席解释限定在 GetTask 调用，而非 schtasks 通用退出码。此映射仍需真实目标系统的正负向验收。

存在性是时间点观察，不消除查询后竞争。下一步注册必须使用 TASK_CREATE 排他创建并重新读回，不得采用先查缺席再覆盖任务。UX-003 保持 doing，Windows 启停/注销及原生旅程未完成，远端保持不变。
