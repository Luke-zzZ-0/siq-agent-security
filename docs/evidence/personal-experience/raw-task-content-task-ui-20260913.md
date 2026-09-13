# M120 任务详情原文授权与二次查看界面

日期：2026-09-13；任务：UX-013；规格：§3.12.29、ADR-048。

本批把已完成的原文 Grant/Revoke 与记录管理协议接入有可信 Binding 的任务活动详情。未归属活动不渲染该面板。前端计算当前 task_id 的 sha256 引用，并在展示前核对 Grant 与 Record 的完整合同及任务关系；不在浏览器内冒充服务端密码学验签。

完成行为：

- 创建授权要求显式选择输入、参数、结果或备注，设置 10 分钟至 24 小时有效期、不超过仓上限的保留期、单条大小及当前人工身份，并勾选任务范围确认。
- active Grant 可通过独立确认撤销；撤销请求绑定页面刚读取的完整 Grant 签名。expired/revoked 只读展示。
- 记录清单默认只展示种类、状态、时间、大小、排除 secret 数量和记录 ID，不预取明文。
- 用户选择一条 active 记录后还需二次勾选，才发起一次 read；返回记录必须与所选 task_ref、record_id 及元数据完全一致。关闭、刷新、切换动作或失败会撤下明文。
- active/expired 记录都可选择删除，但必须明确确认完整 record_id；成功响应必须确认同一 ID，随后重新读取清单。
- 授权、撤销、读取和删除互斥进行；启动其他动作前清除现有明文和竞争确认。原文、记录 ID 和 Grant 签名不写 localStorage/sessionStorage，也不增加下载或复制入口。

浏览器旅程使用实际 Go daemon、实际签名 Activation、实际 Grant 创建与终态撤销；任务详情及合成 Record 的元数据/read/delete 响应使用固定合同替身，从而单独验证“打开确认不读取、二次确认后恰好读取一次、关闭移出 DOM、完整 ID 删除确认、跨任务响应失败关闭”。M118 已独立覆盖真实服务的密文读取/删除 API；本批不把前端替身描述成真实密文采集。9 项检查通过且无 page error：

- [结构化结果](raw-task-content-task-ui-20260913/browser/result.json)
- [桌面任务原文面板](raw-task-content-task-ui-20260913/browser/raw-task-panel-desktop.png)
- [移动端授权区域](raw-task-content-task-ui-20260913/browser/raw-task-panel-mobile.png)
- [移动端授权与记录摘要](raw-task-content-task-ui-20260913/browser/raw-task-records-mobile.png)

验证通过：

- Web：21 个测试文件、70 项 Vitest；个人版和企业版 TypeScript/Vite 构建。
- 固定 Go Grant、Revocation、Record、Content、Delete 样例通过 4 项新增前端合同/关系测试；伪造字段、重复 ID、状态关系及跨任务摘要均拒绝。
- Browser：实际 Grant/Revoke 与固定记录 UI 旅程；两份浏览器脚本 Ruff 通过，390px 无横向溢出。
- Go：`go test ./...`、`go vet ./...`，`rawcontent`、`server`、`ui` race；最终 embed 另经 `internal/ui` 测试。
- 合同：196 项 Python Draft 7 合同测试和 Ruff 通过。
- 最终 `CGO_ENABLED=0` 四目标构建：`linux/amd64` `c47c3ab40d3a9153743a815c44faf0583c586b0b95292305a8bbb28fbbc22795`、`linux/arm64` `437e57d635190d153b3b62b75f5214674265d855018ccd38ce06b101bfe95e99`、`darwin/arm64` `9d3798b3ee0d76078ee42bc547f201e428268463c969d1262ff75fb2ac0134d1`、`windows/amd64` `e0dd172f3c89e8fc2183bbe9b4d31dcf11ccc175d076cb732363287e3e3ae867`。
- `git diff --check` 无输出。

边界：原生 Hermes/OpenClaw/WorkBuddy 适配器尚未调用 CapturePermit/Capture，因此该界面中的授权不等于平台已经采集。自动定时清理、诊断/导出包原文排除验证、原生端到端样例及真实 macOS/Windows 综合验收仍待完成。截图只包含合成元数据，不包含合成明文。本批未提交、推送或合并。
