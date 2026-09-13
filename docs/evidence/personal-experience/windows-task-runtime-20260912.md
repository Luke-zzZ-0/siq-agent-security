# M80：Windows 任务运行状态读回

日期：2026-09-12。规格 §3.11.42；`task-runtime` 只读命令，本地后继增量基于 274ed97。

完整配置查询前后夹住当前 SID/任务的 State、GetInstances(0).Count 和 LastTaskResult 读取；脚本额外核对字段读取前后 State 相同。Go 拒绝未知/禁用状态、非单实例数量和明显矛盾组合。仅接受 ready/0、running/1、queued/0，不把查询失败视为空闲。

内部响应严格分段、规范十进制，保留 LastTaskResult 的有符号/无符号 32 位表示。CLI 输出 state、instances、last_result，无新增持久化文件或 JSON 合同。

## 验证

- Windows 定向、Go 全量/vet/CLI race（6.105 秒）、四目标构建通过。
- 六项临时状态/签名模拟读取：ready、running、queued、初始配置漂移、最终漂移、运行查询失败。漂移或失败均不返回已确认结果。
- 协议正向含两种返回码表示边界；负向含未知/禁用状态、运行但零实例、ready 但有实例、多实例、排队与实例矛盾、负数量、前导零、显式正号、负零、尾随内容及返回码越界。
- 没有 Windows/PowerShell 宿主，未原生解析或运行脚本；编译不证明实际 GetInstances/State/LastTaskResult 序列化行为。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | f01598694a7320ca002ea9f0a3341abc8a80d7d3f1986dea8226397770b82423 |
| linux/amd64 | ef8cd65b388867748455784ff75355c3328001bbe1ba5b950ae5bc0666029568 |
| darwin/arm64 | aaa77ec38db7eea28a56f4644823bbabd2e935ae39d78aaf489c35cae5b3d6a0 |
| windows/amd64 | 02d099143c6caeba25e2e7c8c3a0b71da54e4af436245d98afbdb6244c40df2a |

## 范围与后续

[GetInstances](https://learn.microsoft.com/en-us/windows/win32/taskschd/registeredtask-getinstances) 的实例可见性受用户安全上下文限制；本实现前后要求当前 SID 和 LeastPrivilege 的完整配置匹配，不能推广为任意任务的全机进程查询。[State](https://learn.microsoft.com/en-us/windows/win32/taskschd/registeredtask-state) 与 [LastTaskResult](https://learn.microsoft.com/en-us/windows/win32/taskschd/registeredtask-lasttaskresult) 是不同观察，后者不证明当前服务已退出。

现有 serve 退出依赖 SIGINT/SIGTERM；下一步为 Windows 后台补当前运行绑定的正常退出请求，再结合任务空闲、主 Writer 释放与本次退出结果核对。不能把 Task Scheduler 强制结束或历史零返回码计为本次正常排空。状态前后相同也不是系统原子快照，仍需原生并发/退出验收。

UX-003 保持 doing，未提交、推送或合并功能增量。
