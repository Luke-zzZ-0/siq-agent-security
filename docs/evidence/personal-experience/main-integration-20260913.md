# 全分支整合记录（2026-09-13）

用户已明确授权合并所有分支到 main。基线为远端 main `b6f186d`，整合分支 `codex/all-branches-main-20260913`。整合对象为本轮 fetch 时所有本地与 origin 分支头，以及开放 PR #25/#26/#29/#30/#31 的 head；外部分支仅获取这些已提出 PR 的 head，不操作外部仓库。

- M48–M132 与审查修复经 `e12b323` 纳入，保留生产 Git 获取拒绝、权限与通知安全修复。
- 历史 Pages 刷新、签名实现/状态原来以 squash 方式进入 main；本次连接其原分支历史。冲突处保留 main 后续暖纸墨蓝样式、当前 README 和签名已验证状态，没有回退为旧“待签名”。
- 纳入四场景研究证据、架构图完整展示/缩放、Skill 分发兼容工具和可选 SonarCloud 配置；不等同新产品跨平台验收，不配置外部服务 token/启用变量。
- `gh-pages` 是无共同祖先的生成站点分支。其完整树放入 `docs/archive/gh-pages-9a4ebdc/`，并作为归档合并的父提交保留；原树与归档子树均为 `7a89dad2e55ccdf86784ef8696ecbd51adb355a0`。不覆盖 `site/` 和源码根目录。原第三方 vendor 字节与校验值保留，使用限定路径的 whitespace 属性避免改写历史制品。
- [分支清单](main-integration-20260913/branch-inventory.json) 中全部 captured head 已是整合分支祖先。保留分支，不删除分支或标签。
- GLM 共享工作目录仍在原分支，其大部分未提交代码已通过独立提交纳入；相对 M132 提交剩余四项历史记录/交接差异见 [未提交差异](main-integration-20260913/uncommitted-remainder.json)，不擅自覆盖。GLM 后续从更新后的 origin/main 新建工作树，不将原目录整批再次提交。

整合本地验证：Go 全模块通过；Web 73 项与 build 通过；研究脚本 unittest 66 项通过；Python 研究+合同测试通过；站点链接/发布身份、Actions 固定引用和 Mermaid vendor 校验通过。日志同目录。远端合并结果与 CI 以对应 PR 和 main 提交为准。
