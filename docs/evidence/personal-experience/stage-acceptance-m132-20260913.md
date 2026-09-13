# M132 独立阶段验收（2026-09-13）

结论：**changes_requested，1 项 P1、2 项 P2。新版检查核心/API 已实现，尚未通过独立验收，UX-010 保持 doing。** 本次仅审查已完成的 M132，并检查它调用的 M131 Git 获取边界。没有修改 GLM 的实现或开发台账，没有提交、推送、发布或重启服务。

从共享工作树冻结至 `/tmp/siq-m132-review-20260913` 后检查、运行测试；审查结束时六个关键实现文件仍与共享工作树一致。文件摘要、测试输出与负向补丁见 [验证目录](review-m132-20260913/verification.json)。

## 必须处理的问题

### R132-01 · P1：Git 来源没有执行与 ZIP 相同的目标地址约束

位置：`apps/agentshield/internal/skillimport/git.go:131`、`git.go:137`、`git.go:194`；M132 经 `upstream.go:73` 复用该路径。

`downloadURL` 对域名做格式校验，对 IP 字面量做公网校验；它不解析域名并验证实际地址。Git 随后自行解析/连接，没有走 ZIP 的 `archiveFetcher.connect`，也没有连接地址固定和逐跳目标验证。HTTPS-only 和禁止 file 协议不能替代公网地址约束。

独立负向测试使用本机解析到 127.0.0.1 的 `localhost.localdomain`，替换为不联网的假 Git 程序。生产 `fetchGitCLI` 将该 URL 交给了 Git，未在启动前拒绝；返回 download_failed 而非 url_blocked。测试失败，无跳过。此证据证明目标校验缺失，不声称已经读取内网 HTTPS 内容。此前尝试真实本地监听受低端口权限限制，最终复现不依赖监听，也不访问用户服务。

这是 M131 引入、M132 继承的安全缺口，不能仅作为“未联测”带过。建议先统一 Git/ZIP 的可连接地址约束：解析结果全部校验、实际连接固定、重定向逐跳校验；仅预先 DNS 查询仍无法解决重绑定。若当前 Git 传输无法满足这些条件，应明确拒绝该来源的获取，不能继续报告它满足公网限制。

### R132-02 · P2：上游不可用被误报成安装内容变化

位置：`apps/agentshield/internal/skillinstall/update_check.go:92`、`stage.go:183`；HTTP 映射见 `internal/server/skill_install.go:65`。

`CheckUpdate` 将获取失败交给既有 `sourceError`，后者除了取消和限额之外一律返回 `ErrChanged`。当 Git 不存在或上游下载失败时，接口因此返回 409 `skill_install_changed`。用户无法区分“当前不能检查”与“本地记录/来源已发生变化”，后续重试和提示也无法正确处理。

补充测试分别注入 `skillimport.ErrUnavailable`、`ErrDownloadFailed`，两项均复现得到 `skill_install_changed`。建议为新版检查定义独立、明确的错误分类：不可用/获取失败、URL 被拒、来源绑定变化、限额、取消；保留既有来源摘要不匹配的冲突语义。添加 HTTP 层断言，不要为了新版检查直接改变所有旧调用方的错误含义。

### R132-03 · P2：新增接口缺少合同事实源和实现规格

位置：`apps/agentshield/internal/skillinstall/update_check.go:20`、`update_check.go:26`、`internal/server/skill_install.go:233`。

新增请求 `local-skill-update-check/v1` 和响应 `local-skill-update-check-result/v1`，但 `packages/contracts/` 没有对应 Schema，Go 合同样例和 Python 合同测试也没有登记，`docs/agentshield-dev-spec-v1.md` 未描述新接口。现有 197 项合同测试通过不能证明新接口已受合同验证。

建议先补规格和两个合同，明确必填键、额外字段拒绝、git/zip 字段关系、状态与 requires_confirmation/差异数量的关系、200 条上限、截断标志与错误分类，再添加真实输出样例和正负向校验。相邻 M131 Git 导入请求合同也应随依赖验收补齐。

## 已确认与验收边界

- 已有上游快照、内容差异、200 条报告预算、管理员能力与严格 JSON 测试通过。检查结果不会创建授权，也不会自动更新已安装 Skill。
- 重新执行三个相关包原有测试通过；`go vet ./...` 通过；全模块原有测试通过（`go test -count=1 -skip '^TestReviewer' ./...`）。单独执行 reviewer 负向测试，两组失败是上述缺陷的复现证据，不能计为整体通过。
- 本次没有重新执行四目标构建或原生平台验收；GLM 的既有构建摘要保留为其报告，不标成本次独立实测。
- `apps/web/src` 尚无 update-check 的请求封装或入口，所以本批仅完成核心/API。用户从管理界面发现新版、查看结果和进入确认流程尚未接通，不能写成用户可使用的完整新版检查。
- “不写状态”应准确表述为“不新增或修改持久业务记录/授权；允许一次性下载暂存”。`CheckUpstream` 确实在 skill-imports 下创建临时目录，退出时清理。现有 skillinstall 路径清单测试仅排查新增路径，不能证明既有文件未改写或删除；建议补整个状态目录的文件内容/权限摘要前后比对，以及失败、取消时的暂存清理断言。
- 报告写“210→200 截断”，当前测试实际构造 `maxInspectionChanges+1`，即 201→200。应按实际用例更正文档；这是证据精度问题，不另计运行时缺陷。

## 接续顺序

1. 先修 R132-01 获取边界及 R132-02 错误分类，补对应负向与 HTTP 回归。
2. 补 R132-03 规格/合同/样例，再接通个人控制台检查入口与结果状态。
3. 增强无持久写入与取消清理证据；重新验收 M131/M132。
4. 继续通用旧状态写入拒绝、原生更新验收。跨 OS、WorkBuddy、可信 Skill 来源和 LAN 目标不因本批测试通过而提升状态。

负向用例只保存在 [补丁](review-m132-20260913/reviewer-negative-tests.patch) 和隔离快照中，未向共享源码添加失败测试，便于 GLM 在修复分支应用。
