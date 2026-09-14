# ADR-0051: 受控公共托管来源的 Skill Git 导入

日期：2026-09-13
状态：设计接受、组件实现已验；生产入口仍关闭，待固定候选的真实 HTTPS 成功与受控失败验收。

## 背景

任务书 N01 合并后，Skill Git 导入的生产路径始终以 `ErrGitTransportUnavailable` 拒绝：git CLI 自身的传输无法满足固定公网地址校验、有界获取与确定性归档要求，而"直接打开 git clone"被任务书明确排除。用户因此只能使用 HTTPS ZIP 或本地导入。N02 要求设计并实现一种可验证的安全生产获取路径，使 Git 导入真正可用，同时不放宽任何既有硬边界。

## 决策

不使用 git 协议（smart HTTP、pack 协议、git CLI 传输）。取而代之，对显式准入的公共托管方采用"受控 HTTPS 元数据解析得到不可变 commit，再按该 commit 取得受限源码归档"的两段式获取：

1. **来源定位符解析。** 用户 URL 必须形如 `https://github.com/{owner}/{repo}`（允许一个尾部 `/` 与 `.git` 后缀；host 经既有 `downloadURL` 规范化后必须精确等于 `github.com`，禁止端口、userinfo、query、fragment）。owner 为 1–39 个字符、字母数字开头、不含连字符结尾；repo 为 1–100 个字符的 `[A-Za-z0-9._-]` 且不为 `.`/`..`。不满足时返回稳定错误 `skill_import_git_host_unsupported`（不支持来源，指引改用 HTTPS ZIP 或本地导入）或 `skill_import_url_blocked`（受支持 host 上的病态路径）。
2. **ref 解析为不可变 commit。** 对 `https://api.github.com/repos/{owner}/{repo}/commits/{ref}`（ref 为空时用 `HEAD`，即托管方默认分支）发起 GET，响应限 1 MiB、仅接受 200、**禁止重定向**，从严格 JSON 中提取 40 位十六进制 `sha`。ref 语法沿用既有 `gitRefValid` 白名单（无选项、无 `..`、无前导点段）。解析失败、ref 不存在、仓库不存在、5xx 一律返回新的稳定错误类 `skill_import_source_unavailable`。
3. **按 commit 取归档。** 对 `https://codeload.github.com/{owner}/{repo}/zip/{sha}` 发起 GET，**禁止重定向**，体积受既有 `maxArchiveBytes`（32 MiB）约束。因为 URL 中是已解析的 commit 而非 ref，第 2 步与第 3 步之间远端分支漂移不会改变取到的内容；记录中的 `commit_sha` 即该响应所对应的内容。
4. **落盘复用既有有界管线。** 归档经 `checkZipDirectory` + `extractZip`（预算、zip-slip、符号链接、git 元数据排除全部沿用）解包到私有临时目录，要求恰好一个顶层目录、零个顶层文件（codeload 归档的固有形态，兼作内容形态校验），再经 `directoryTree` 复制进暂存工作树，使 `SubDir` 语义与快照/准入复核完全不变。

网络边界完全复用 ADR-035 的 HTTPS 传输：每次拨号前逐 IP 公网校验（DNS 重绑定防护）、IANA 特殊用途段排除、TLS 1.2+ 主机名校验、代理禁用、单请求 45 秒总超时；受控请求额外把重定向上限设为 0。错误分类：策略违规 → `skill_import_url_blocked`；来源侧不可用/畸形 → `skill_import_source_unavailable`；网络/TLS 失败 → `skill_import_download_failed`；超限 → `skill_import_limit`。

`ErrGitTransportUnavailable` 随本决策移除：它描述的"通用 git 传输暂不可用"状态不再存在，生产 fetch 默认实现改为上述受控托管获取；git CLI 克隆仅保留为测试夹具（`file://` 经私有 seam 注入），运行期配置无法启用任何传输缝。

## 后果

- 首批仅 `github.com` 公共仓库可用；其他 host（含自建 GitHub Enterprise、GitLab 等）得到稳定、可测试的拒绝与替代路径提示。逐个准入新托管方只需扩展定位符解析与两端点模板，不改变安全机制。
- 记录契约 `local-skill-import-git-create/v1` 与 `local-skill-import/v2` 的 `GitMetadata` 无需变更：`commit_sha` 继续钉住不可变内容，`expected_commit` 继续提供安装方比对。
- API 与 codeload 归档均无凭据、无私有仓库访问；仅显式列出的公共端点可被请求。速率限制表现为 `skill_import_source_unavailable`，不会重试风暴。
- 对不支持来源的拒绝稳定且可测试；上游检查（`CheckUpstream`）按同一受控路径重新解析 ref，漂移如实上报为"未钉住"。
