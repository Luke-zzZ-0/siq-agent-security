# M98：活动详情实际效果核验展示

日期：2026-09-12。规格 §3.12.7，本地未提交增量。

详情加载后以同一活动和快照请求结果核验，面板与回执分开呈现。要求完整活动绑定匹配、有效评估时间；verified 必须有非空要求、每项 verified 且非空证据、无 incident。支持未知归属/意图缺失/未定义要求说明；请求或校验失败清除旧结果，不把回执放行当作实际效果已核验。

## 验证

- `npm test`：15 文件、52 项测试通过。消费 Go 共用 completion fixture，覆盖未知不升级、缺意图、错误归属、无证据 verified、跨任务/快照/会话、非法 completed、incident 与局部未知阻断顶层 verified。
- 企业与本地构建（含 TypeScript）通过；构建 `/tmp/siq-m98-browser` 集成 embed。
- 浏览器脚本通过：`python3 scripts/personal-experience/task-activities-browser-smoke.py --binary /tmp/siq-m98-browser --out-dir /tmp/siq-m98-browser-evidence`。真实隔离服务启动/配对/空列表；内容与效果响应使用明确 fixture。验证成功显示效果要求及 evidence-fixture-1，随后 completion 500 移除旧 verified、仍能查看裁决回执，再次刷新可恢复；原有详情 404、返回、列表错误旅程继续通过，无 pageerror。
- 桌面与移动详情截图已生成，移动结果面板检查通过；输出位于 `/tmp/siq-m98-browser-evidence/`。
- `git diff --check` 通过。

## 边界

浏览器 fixture 用于展示与状态测试，不是效果已原生发生的证据。真实文件材料与后端判定验证见 M97，三系统/三平台原生验收仍独立待办。证据引用目前是只读标识，证据详情、可信 Skill 版本与导出仍待接入；UX-011 不标完成。未提交或推送。
