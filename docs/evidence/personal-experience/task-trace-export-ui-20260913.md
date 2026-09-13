# M112 完整追溯包前端入口

日期：2026-09-13；任务：UX-011/012/013；规格：§3.12.21。

已归属活动详情新增“下载完整脱敏追溯包”，与“下载脱敏回执摘要”分开呈现。完整包说明包含回执摘要、历史 Skill 来源状态、完成结论和被引用的证据元数据，并明确浏览器只核对格式和字段关系；密码学验签仍需外部可信公钥。下载成功显示包内材料是否 incomplete，未归属活动不显示下载入口。

前端响应校验采用严格字段白名单，核对活动 ID、快照、整组回执范围、来源 seq/hash、来源状态、完成结论约束、incomplete 等价关系、效果元数据枚举和全部引用集合。未知字段、缺失/多余证据、重复引用、错误来源、矛盾完整状态或旧快照均不保存。409、413 与通用失败使用不同提示；组件卸载会取消请求。

验证通过：

- `npm test`：19 个文件、63 项测试；固定 Go 追溯样例和来源/完成/证据关系负向通过。
- `npm run build` 与 `npm run build:local`；日志 `/tmp/siq-m112-enterprise-build.log`、`/tmp/siq-m112-local-build.log`。
- 本地 embed 二进制 `/tmp/siq-m112-browser` 构建通过；隔离 Chromium 脚本完成真实管理配对及原有活动旅程。
- 明确 fixture 验证点击前不请求、下载名和内容、incomplete 状态、快照冲突及导出区仅有一个 DOM 节点；桌面和 390 px 移动截图人工检查通过，无 page error。
- 浏览器证据在 `/tmp/siq-m112-browser-evidence-final`，结果继续标记 `native_platform_acceptance=false`。

限制：浏览器保存的是服务端签名文档，但自身没有外部信任锚，因此不宣称已完成密码学验签。活动和追溯内容是明确 fixture，不作为 OpenClaw、Hermes、WorkBuddy 或 macOS/Windows 原生证据。真实平台任务样例与保留期联动继续待办。本批未提交、推送或合并。
