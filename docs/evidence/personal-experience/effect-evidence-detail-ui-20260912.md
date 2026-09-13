# M99：效果证据元数据详情

日期：2026-09-12。规格 §3.12.8，本地未提交增量。

复用现有 GET /v1/effect-evidence/:id，服务端 Store.Get 验签。UI 从已核验 completion 的证据/事件引用提供按需打开按钮，cache=no-store；核对返回证据 ID/任务及元数据结构，只投影来源、独立性、覆盖范围、状态、引用和摘要。签名只做格式检查，不声称浏览器执行密码学验证。

## 验证

- `npm test`：16 文件、54 项测试通过。使用共享 effect-evidence 样例验证投影与任务/证据身份、非法来源/范围/时间/摘要/签名格式拒绝；附加原始观测内容不会被投影。
- 企业/本地构建及 TypeScript 通过，重新构建 `/tmp/siq-m99-browser`。
- `python3 scripts/personal-experience/task-activities-browser-smoke.py --binary /tmp/siq-m99-browser --out-dir /tmp/siq-m99-browser-evidence` 通过。断言点击前无证据请求，打开后能看到来源和宿主独立观测，读取失败有提示，关闭后焦点返回按钮；已有核验失败去除旧成功、详情与列表旅程继续通过，无 pageerror。
- 证据桌面截图已检查，详情/列表移动截图仍由脚本生成。输出在 `/tmp/siq-m99-browser-evidence/`。
- git diff --check 通过。

## 边界

浏览器证据详情为明确的响应 fixture，包括结构测试用包装签名；不把此截图当作密码学或真实智能体执行证明。密码学验证复用已有服务端测试，真实文件效果验证见 M97。单份 completed/expected 不等于任务 verified。可信 Skill 版本关联与脱敏导出仍待完成；UX-011 进行中，未提交或推送。
