# M75：Windows 任务配置读回核对核心

日期：2026-09-12。分支 `codex/personal-macos-stop-recovery`，基于 `274ed97` 的未提交增量。规格 §3.11.37；实现 `windows_task_verify.go`，使用标准库 XML 解析，无系统副作用。

核对完整 XML 树，保留命名空间、非 xmlns 属性与叶节点原文，允许格式空白、属性顺序和本配置中单值子元素顺序的差异。拒绝新增/缺失字段、重复属性/元素、额外 Exec 或触发器、账户/权限/参数/设置漂移。未知系统字段不能静默忽略；后续原生兼容差异需要逐项验证。

格式依据 [Microsoft Task Scheduler Schema](https://learn.microsoft.com/en-us/windows/win32/taskschd/task-scheduler-schema)。这里只核对 SIQ 固定配置的归属，不实现通用 Task Scheduler XSD 验证器。

## 验证

- Go vet、全量测试和 CLI race 通过；race 用时 5.211 秒。中断后追加边界测试并重跑 Windows 定向测试通过；未修改运行时代码。
- 正向包含共用 XML、格式变化、注释、属性和设置顺序调整；负向覆盖账户提权、其他 SID、登录方式变化、强制终止、额外动作、触发器、未知/缺失/重复设置、重复/额外属性、命名空间变化、参数变化、混合内容、DTD、处理指令、截断与尾随内容。
- 长度 64 KiB、深度 16、元素 256 的精确上限通过，上限 +1 拒绝。补充外来命名空间叶节点、未绑定前缀、命名空间属性注入、重复命名空间、非法 UTF-8、未经转换的 UTF-16 声明与第二 XML 声明拒绝。
- 四目标构建通过，实际文件 SHA-256 如下。追加测试不改变二进制源码。

| 构建 | SHA-256 |
| --- | --- |
| linux/arm64 | adfa1015b131cf7f06f94f424144aa0265ef5798a90d954e883de2a49dcea4a0 |
| linux/amd64 | 468f46aa2d8df493ded6eb6d47f193fab7024530e09e189c88123caa8bb4b3b8 |
| darwin/arm64 | b6217701adc274b2af646cf1c2433ac51853154544a345f49ea127cc50d0911c |
| windows/amd64 | 23677e2913851b448915b9cc8889d68221db213bb87820510c31615db5cf4520 |

## 尚未覆盖

本批没有查询 Windows Task Scheduler，不证明某任务存在或缺席，也未接通注册/启动/停止。输入暂为 UTF-8；系统查询传输层仍需处理实际编码、完整输出、超时及查询失败。核对核心尚未接入 CLI，不能单凭该函数声明 Windows 后台支持。Windows 原生实机验收保持待办。

README 更新已单独提交并推送至 `codex/readme-personal-status-20260912`（`b6f186d`），不包含本批功能代码。PR #31 保持原提交；本批未推送或合并。
