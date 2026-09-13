# 全分支整合记录（2026-09-13）

用户已明确授权合并所有分支到 main。基线为远端 main `b6f186d`，整合分支 `codex/all-branches-main-20260913`。整合对象为本轮 fetch 时所有本地与 origin 分支头，以及开放 PR #25/#26/#29/#30/#31 的 head；外部分支仅获取这些已提出 PR 的 head，不操作外部仓库。

- M48–M132 与审查修复经 `e12b323` 纳入，保留生产 Git 获取拒绝、权限与通知安全修复。
- 历史 Pages 刷新、签名实现/状态原来以 squash 方式进入 main；本次连接其原分支历史。冲突处保留 main 后续暖纸墨蓝样式、当前 README 和签名已验证状态，没有回退为旧“待签名”。
- 纳入四场景研究证据、架构图完整展示/缩放、Skill 分发兼容工具和可选 SonarCloud 配置；不等同新产品跨平台验收，不配置外部服务 token/启用变量。
- `gh-pages` 是无共同祖先的生成站点分支。其完整树放入 `docs/archive/gh-pages-9a4ebdc/`，并作为归档合并的父提交保留；原树与归档子树均为 `7a89dad2e55ccdf86784ef8696ecbd51adb355a0`。不覆盖 `site/` 和源码根目录。原第三方 vendor 字节与校验值保留，使用限定路径的 whitespace 属性避免改写历史制品。
- [分支清单](main-integration-20260913/branch-inventory.json) 中全部 captured head 已是整合分支祖先。保留分支，不删除分支或标签。
- GLM 共享工作目录仍在原分支，其大部分未提交代码已通过独立提交纳入；相对 M132 提交剩余四项历史记录/交接差异见 [未提交差异](main-integration-20260913/uncommitted-remainder.json)，不擅自覆盖。GLM 后续从更新后的 origin/main 新建工作树，不将原目录整批再次提交。

整合本地验证：Go 全模块通过；Web 73 项与 build 通过；研究脚本 unittest 66 项通过；Python 研究+合同测试通过；站点链接/发布身份、Actions 固定引用和 Mermaid vendor 校验通过。日志同目录。远端合并结果与 CI 以对应 PR 和 main 提交为准。

## 整合 CI 修复

首轮 PR #32 暴露三项问题，均在整合分支修复：

1. gitleaks Action 将独立 Pages 根提交拼成不存在的 `root^..HEAD`，未能扫描。改用校准步骤已安装的固定版本 8.24.2 CLI，扫描 `--full-history -m HEAD`；不降低默认规则，不增加例外。新增真实临时 Git 历史负向校准，验证当前树已删除的根提交、独立分支和仅在 merge 新增的合成凭据仍被发现。整合历史 398 提交、约 228 MB 扫描无泄漏。
2. PR #29/#30 的研究工具修复 Ruff 导入顺序、上下文管理器、预期失败子进程的 `check=False` 和入口可执行位；66 项 unittest 与 Ruff 复验通过。
3. MCP 组件桥旧脚本跳过前置授权，新版 Hermes 正确拒绝关联。验证脚本使用明确映射为既有 `read_file` 的 MCP 夹具，先签发 USER 路径来源、取得真实 pre-tool 允许，再读取合成报告和提交 post-tool 结果。补充未关联结果不可获取来源引用的负向断言。未修改生产适配器，未放行未知 MCP 效果，不宣称原生宿主验收。

最终远端检查与合并身份以 PR #32 最新 head 为准；本地日志不替代远端 CI。

## 远端结果

PR #32 最终 head `cbf44bea1bcf38ff4286bc1ae0a59e17b4ccd670` 的 39 项检查通过，3 项按条件跳过（PR 不部署 Pages、未配置可选 SonarCloud、非 nightly 事件）。按用户明确授权，以管理员合并方式满足本次合并要求，未改动分支保护或仓库设置。2026-09-13 16:08:44（Asia/Shanghai）合入 main `983b820ce85efe5902f33af6d4bb16f88c83ba82`。所有开放开发 PR 已随历史整合关闭，分支继续保留。机器可读状态见同目录 `pr32-final.json`。
