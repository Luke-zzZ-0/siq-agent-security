# M94：个人任务活动页面

日期：2026-09-12。规格 §3.12.3，本地未提交增量。

新增 /activities 导航、任务/未归属视图和分页。视图、offset、snapshot 在 URL，刷新按钮重新从第一页读取；响应校验请求视图、偏移、快照、归属关系与计数，不合格响应不展示。Effect 清理取消请求并忽略旧响应，加载或失败时隐藏上一页内容及完整性结论。页面不声称 allow 等于效果已核验。

## 验证

- `npm test`：14 个文件、49 项测试通过。新增测试消费 Go 共用合同 fixture，覆盖跨视图/offset/snapshot、伪造完整性、分页、未归属与非法绑定。
- `npm run build`、`npm run build:local` 通过（含 TypeScript）。最终操作栏样式调整后再次构建本地前端和 Go embed 二进制。
- `python3 scripts/personal-experience/task-activities-browser-smoke.py --binary /tmp/siq-m94-browser --out-dir /tmp/siq-m94-browser-evidence` 通过：隔离临时状态的真实服务配对、真实空活动列表；随后仅模拟活动 API 响应，测试已归属、未归属、URL 刷新恢复、409 快照变化、500 不可用、错误隐藏旧记录及恢复。没有 pageerror。
- 1440×1000 桌面与 390×844 移动截图已检查。初次移动截图捕获侧栏收起动画，脚本增加布局状态等待后复验；操作栏用既有 toolbar 处理换行，视图按钮标明选中状态。
- `git diff --check` 通过。

浏览器输出位于 `/tmp/siq-m94-browser-evidence/{result.json,desktop.png,mobile.png}`，复现脚本在仓库内。内容与异常场景是响应 fixture，不是 Windows/macOS 或真实智能体执行证据。

## 后续

当前用户可查看活动分组，但尚无任务详情、分页回执内容、结果核验和可信 Skill 版本联动；保留原始回执入口，不将列表当作完整 UX-011 验收。功能未提交、推送或发布。
