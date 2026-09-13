# M78：Windows 用户任务排他注册

日期：2026-09-12。规格 §3.11.40；后继分支基于 274ed97，功能仍未提交。命令 `task-register --confirm-register`。

命令复用签名准备，持生命周期与主 Writer；检查 pending 切换和归属后查询存在性。已有任务只复用完整匹配配置；缺席则复验源后调用固定脚本 RegisterTask，使用 TASK_CREATE=2、当前 SID、空密码、InteractiveToken=3。系统同名竞争失败保留现场，无覆盖、启动、删除或更新操作。成功必须经过独立系统配置读回和最终源复验。

脚本运行复用系统目录 PowerShell、固定 EncodedCommand、stdin JSON、有界输出、超时和 stderr 拒绝。输入中的 XML 只作为 COM 参数，不作为脚本执行；主程序提供已签名且固定渲染的配置。

## 验证

- Windows 定向、Go 全量/vet、CLI race 通过（race 5.619 秒）；四目标构建通过。
- 九项模拟编排：创建、已有匹配复用、存在性失败、已有异配置、竞争创建失败、创建后读回失败、创建后配置漂移、存在性查询期间源改变、无归属。断言调用顺序及创建次数，错误没有成功输出；失败后本地签名配置仍可核验。
- 缺少确认、明确 false、额外参数拒绝。
- 代码审阅固定 TASK_CREATE 参数，并对照 Microsoft RegisterTask/TASK_CREATION 文档；模拟控制器不能证明实际 Windows 的排他行为。
- 当前没有 Windows/PowerShell 宿主，未解析或执行注册脚本，未验证用户账户、任务 ACL、系统序列化与真实竞争注册。编译不计为原生注册验收。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | 6ee5397c3627753b3790b91b5fc24282fbf892bf1b6875217c015e2517c0bcf9 |
| linux/amd64 | 77357779c52a64cc1537175531a24dae87b4ad9d0d488950fe50143efa9c887f |
| darwin/arm64 | 11c0aa76a0cd5027a2d12370d0e9584b8eb5c587f5b9e0396f6a1947c020b46b |
| windows/amd64 | d2f914f4c9693f23564e4840c356b03ea7621b9ef2715824e407cd6eac1f5efd |

## 依据与后续

接口依据：[TaskFolder.RegisterTask](https://learn.microsoft.com/en-us/windows/win32/taskschd/taskfolder-registertask)、[TASK_CREATION](https://learn.microsoft.com/en-us/windows/win32/api/taskschd/ne-taskschd-task_creation)。采用 CREATE，拒绝以 UPDATE 或 CREATE_OR_UPDATE 代替排他创建。

系统创建成功但读回失败可能留下任务；不自动删除，重试必须重新核对归属。当前不调用 Run，Windows 启动、健康核对、正常退出与注销仍待实现；原生注册及恢复旅程待验收。UX-003 保持 doing。未改变远端 PR 或发布制品。
