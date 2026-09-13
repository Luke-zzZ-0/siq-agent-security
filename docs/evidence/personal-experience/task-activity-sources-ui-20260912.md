# M107 个人活动历史 Skill 来源面板

日期：2026-09-12；UX-011/012；规格 §3.12.16。

新增 taskSources.ts 和 ActivitySourcesPanel.tsx，接入管理 API 与活动详情。仅在点击展开后读取当前页；校验活动 ID、快照、视图、分页、逐行 seq/hash/Grant 和来源白名单，区分来源已核验/不可用/未归属。展示声明版本与分开的摘要，缺少版本说明未声明；保留“不证明实际执行或当前权限”的说明。

验证通过：

- npm test：18 文件、59 项测试。共享 Go 样例及范围/缺行/错误 Grant/私密字段/空来源/未知状态/导入摘要/未知归属验证。
- npm run build 与 npm run build:local，日志 /tmp/siq-m107-{enterprise,local}-build.log。
- Go embed 二进制 /tmp/siq-m107-browser，隔离脚本 scripts/personal-experience/task-activities-browser-smoke.py 通过，无 pageerror。
- 真实管理配对与空列表后使用显式 fixture；点击前无来源请求，展开可见 history-skill/1.2.3，关闭后模拟 500 再展开，错误可见且旧可信来源消失。原导出/效果/证据旅程继续通过。
- 桌面与 390px 移动端来源截图已检查，长摘要换行；产物 /tmp/siq-m107-browser-evidence。
- git diff --check。

浏览器来源材料为明确 fixture，不计为真实平台验收；M106 API 测试承担本地真实签名核验。这里的关联是历史授权来源，不证明某次调用实际执行该 Skill。筛选、来源/效果导出以及原生平台真实样例尚待推进。未提交推送。
