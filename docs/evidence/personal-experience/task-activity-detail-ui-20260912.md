# M96：个人任务活动详情页面

日期：2026-09-12。规格 §3.12.5，本地未提交增量。

列表增加“查看活动记录”，进入 /activities/:id 并携带视图、快照及来源列表偏移。详情展示本组回执的工具、裁决、原因、时间、授权引用；分页保留快照，404/409/不可用分开提示，刷新重载当前活动。路由变化取消旧请求，加载和错误隐藏旧详情。

## 验证

- `npm test`：14 个文件、50 项测试通过。新增共用 Go fixture 校验活动 ID、分页总数、快照及单条回执主体/会话/平台、seq 范围，拒绝参数摘录字段。
- `npm run build` 与 `npm run build:local`（含 TypeScript）通过；构建 `/tmp/siq-m96-browser` 集成正式 embed 入口。
- 浏览器复现：`python3 scripts/personal-experience/task-activities-browser-smoke.py --binary /tmp/siq-m96-browser --out-dir /tmp/siq-m96-browser-evidence` 通过。真实隔离服务启动、配对与空列表；列表/详情内容及异常采用明确响应 fixture，验证列表进入详情、工具与授权引用可见、404 后隐藏旧记录、返回列表，以及原有未知/刷新/错误旅程。无 pageerror。
- 桌面 1440×1000 和移动 390×844 详情截图已检查；表格沿用横向滚动容器。截图禁用有限动画以捕获最终布局。
- `git diff --check` 通过。

输出含 `/tmp/siq-m96-browser-evidence/detail-desktop.png`、`detail-mobile.png` 和 result.json；仓库中保留复现脚本。

## 后续

当前详情只说明裁决记录，不把放行当作实际效果证明。作用域结果核验、可信 Skill 版本及导出仍待接入。响应 fixture 不计为真实智能体原生执行证据。UX-011 保持进行中；未提交或推送。
