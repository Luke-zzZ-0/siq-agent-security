# M102 个人活动摘要下载入口

日期：2026-09-12；UX-011/012；规格 §3.12.11。

新增 taskExport.ts、ActivityExportButton.tsx，接入现有管理会话 API 与活动详情。只在已归属活动展示入口，点击请求当前活动快照的整组摘要。字段白名单、摘要/签名格式、行数和顺序检查后保存原签名字段；不重签，不声称浏览器验签。错误不创建文件；详情切换/刷新销毁组件并中止旧请求，重复点击受同步请求引用保护。

验证通过：

- npm test：17 文件、56 项测试，复用 M101 Go 固定导出样例；覆盖未知归属、错误快照、缺行、重复/越界行、私密扩展字段、虚假裁决与缺失签名。
- npm run build 和 npm run build:local；日志 /tmp/siq-m102-{enterprise,local}-build.log。
- 本地 Go embed 二进制 /tmp/siq-m102-browser；隔离 Chromium 脚本 scripts/personal-experience/task-activities-browser-smoke.py 通过。
- 浏览器真实管理配对/空列表后使用显式 fixture：点击前无导出请求，下载文件名/JSON 范围正确，409 明确提醒刷新；原活动/效果/证据旅程继续通过。
- 桌面与 390px 移动详情截图已检查，按钮和说明可见；无 pageerror。产物 /tmp/siq-m102-browser-evidence。
- git diff --check 通过。

浏览器导出样例为 UI fixture，修改过范围字段，不计为密码学签名证据；真实 Go 签名及 Python 互验由 M101 测试覆盖。本批没有真实智能体或 macOS/Windows 原生验收。摘要尚不含效果材料/Skill 版本，UX-011 保持进行中。未提交或推送。
