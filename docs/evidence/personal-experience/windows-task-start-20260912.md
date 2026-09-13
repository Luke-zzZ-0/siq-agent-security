# M79：Windows 已注册任务按需启动

日期：2026-09-12。规格 §3.11.41，命令 `task-start --confirm-start`，功能增量基于 274ed97 本地保留。

要求先注册；命令持生命周期锁，核对 pending 切换与完整任务配置。匹配目录的健康服务复用；否则检查主 Writer 可用并先释放，再复验配置，按当前用户调用 RegisteredTask.Run(null)。Run 的成功只表示请求已受理，最终须通过本地 API 目录健康和完整配置复验。失败保留状态与任务，不强制重启、停止或删除。

依据 [Microsoft RegisteredTask.Run](https://learn.microsoft.com/en-us/windows/win32/taskschd/registeredtask-run) 核对调用与动态参数语义；程序和状态目录路径增加 $( 序列拒绝，防止任务动作模板展开改变目标。

## 验证

- Windows 定向、Go 全量/vet/CLI race（6.081 秒）与四目标构建通过。
- 八项模拟编排：正常启动、健康复用、Writer 占用、查询失败、异配置、启动失败、健康超时、最终读回漂移。校验启动次数、Run 前主 Writer 已释放、失败后签名配置保留。
- 无明确确认、false 和多余参数拒绝；$(Arg0)/$(Arg1) 路径负向通过。原有共用 XML 与渲染测试继续通过。
- 10 秒轮询预算另计每次有界系统查询/HTTP 调用，不能宣传为整个命令严格 10 秒截止。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 4a0d7ead7f6f30867005c30d1ccddfd12eb79e087a6167662be2d2fa5d51c7a8 |
| linux/amd64 | 92045139d49d3cc9cc240e1a487af32bcfe7045f864dd612d222bfb1e184bdb2 |
| darwin/arm64 | e0e69ae8678e73a5eb11deaba0747868a1153734f2cc91d4f7781da4f4f33e64 |
| windows/amd64 | d5c75ad02c06f2c487abfd930586043f47f5c335cf11967df4a7e100bce034b2 |

## 原生验收边界

没有 Windows 或 PowerShell 宿主，未解析/运行新脚本，未调用真实 Run；任务引擎进程与业务进程不是同一身份，未用 EnginePID 冒充服务 PID。当前成功表示目录健康与配置匹配；已有健康服务不宣称由本次命令启动。Task Scheduler 实际运行、用户会话、进程关联和三系统安装验收仍待完成。

下一步补可验证的正常退出通道、任务运行状态与注销，再接 Windows setup/teardown。UX-003 保持 doing，功能增量未推送或合并。
