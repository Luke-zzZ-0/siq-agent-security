# M109 个人活动筛选与 URL 状态

日期：2026-09-12；任务：UX-011/012；规格：§3.12.18。

任务活动页新增关键词、平台、智能体、会话和任务筛选。空条件继续读取原列表 API；存在条件时读取 M108 search API。表单使用 URL 作为已应用条件事实源，输入草稿不发请求；提交、清空、视图切换和刷新都会丢弃旧分页快照。后续页携带全部筛选和源快照，详情链接、详情分页/刷新及返回链接保留筛选来源。

前端校验服务端 filters 完整回显、活动范围和分页；URL/表单条件采用与服务端一致的 256 Unicode 字符、1024 UTF-8 字节及控制字符边界。筛选无匹配与读取错误使用不同状态。界面明确关键词仅覆盖任务、主体、会话和平台标识。

验证通过：

- `npm test`：18 个文件、61 项测试。新增共享 Go search 样例验证、视图/条件/快照错配、未知 filter 字段、中文字符边界和控制字符负向。
- `npm run build` 与 `npm run build:local`；日志为 `/tmp/siq-m109-enterprise-build.log`、`/tmp/siq-m109-local-build.log`。
- 本地 embed 二进制 `/tmp/siq-m109-browser` 构建通过；隔离浏览器脚本 `scripts/personal-experience/task-activities-browser-smoke.py` 通过。
- 浏览器先完成真实本地服务管理配对与空列表，再使用明确 fixture：填写草稿时无 search 请求，提交后 URL 带条件并显示匹配数；进入详情、返回和切换未归属视图保留条件；模拟无匹配时不显示旧行，恢复后重新出现；原有快照冲突、读取错误、来源、证据和导出旅程继续通过。
- 桌面及 390 px 移动布局已人工检查；无 page error。浏览器证据位于 `/tmp/siq-m109-browser-evidence-final`。
- `git diff --check` 通过。

浏览器中的筛选数据是显式 fixture，不作为原生智能体平台证据；真实 Go 签名与全快照筛选由 M108 API 测试覆盖。本批未新增 macOS/Windows 原生验收，未提交、推送或合并。
