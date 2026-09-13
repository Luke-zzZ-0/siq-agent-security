# M131：UX-009 增量 —— Git 来源 Skill 导入

日期：2026-09-13。任务书 UX-009（Git 仓库直接导入 Skill）。本地候选基于当前工作区，未提交或推送。

`POST /v1/skill-imports/git`（管理员能力）按 `local-skill-import-git-create/v1` 合同接收 ImportID、URL、Ref、SubDir、可选 ExpectedCommit 与 ActorID，克隆仓库后以与 zip 导入相同的准入管线产出 v2/git 记录。实现要点：

- URL 复用 https_zip 下载校验：仅 https 公网主机、443 或缺省端口、拒绝 userinfo/fragment/反斜杠/控制字符；Ref 白名单正则并拒绝 40 位十六进制（提交以 ExpectedCommit 固定，不作为 ref）；SubDir 复用归档路径校验并拒绝 git 元数据路径。
- git CLI 硬编码（无 shell、无 Go git 库）：环境净化（GIT_CONFIG_NOSYSTEM、空全局配置临时 HOME、GIT_TERMINAL_PROMPT=0、GIT_ASKPASS=echo、HOME/TMPDIR 隔离、LC_ALL=C），argv `-c core.hooksPath=<空目录>`（仓库钩子在 clone/checkout 期间不可执行）、`core.fsmonitor=false`、`gc.auto=0`、`protocol.file.allow=never`、`GIT_ALLOW_PROTOCOL=https`；`clone --depth 1 --single-branch` 后 `rev-parse --verify HEAD^{commit}`，stdout 按四十位十六进制模式校验，stderr 刻意丢弃，远端可控文本不进入本地日志或错误串。
- ExpectedCommit 不匹配 → 409 skill_import_archive_mismatch；克隆在私有 blob/unpacked 暂存完成，沿用目录树限额（2000 文件/目录、深度 16、单文件 8MiB、总量 64MiB）、.git 元数据排除与准入后摘要复查（检查后替换拒绝）。同一 ImportID 不同参数 → 409 skill_import_conflict；重复请求 → reused/200。
- 记录 schema v2 新增 `git` 元数据（URL/Ref/SubDir/ExpectedCommit/CommitSHA）；recordVersionValid 三分支互斥，v1 与 https_zip 记录拒绝携带 git 字段。
- 服务端严格平铺 JSON：所有字段必须作为键出现，缺失/null/布尔/额外键/重复键/大小写不符均 400 skill_import_invalid；槽位 TryLock 429 skill_import_busy；60 秒上下文，超时/取消 → 408。

## 验证

- `go test ./internal/skillimport/ -count=1`：新增 5 项 —— 真实 git fixture 仓库端到端克隆（含恶意 post-checkout 钩子不执行的断言、Exec bit 保留、签名验证、Load 全链路、reused/conflict/wrong-pin/缺 SubDir 负向）；SubDir 子树选择；23 例请求校验表；gitRefValid；记录校验与版本互斥。既有 zip/remote 测试全部保持通过。
- `go test ./internal/server/ -count=1`：新增 `TestGitSkillImportHTTPAdminStrictAndBounds` —— 401/403 凭据边界、私有主机 400 skill_import_url_blocked、GET 405、严格体循环（拼接、null、数组、错误大小写、重复 url 键、额外键、逐字段缺失/null/布尔）全 400 且无错误回显、持锁 429、records 目录保持为空。
- `go vet ./... && go test ./...` 通过；`gofmt -l` 与 `git diff --check` 无输出。
- 四目标构建通过（apps/agentshield，`./cmd/agentshield`，CGO_ENABLED=0）：

| 目标 | SHA-256 |
| --- | --- |
| linux/amd64 | 69983d8b36bf798001537aeba95e8f9f0d56a2ee82f4cf1483ef35d12234c654 |
| linux/arm64 | ac5fad7738263ce79f09c8e28a877966c29d8baebdf9601c48d63df4bdfdc304 |
| darwin/arm64 | 11d6933c4033e882445956e04a00341c90f0a257685f3dc7bf46513e4c4f170c |
| windows/amd64 | f3ad863434571f94266d5abb71a84b224a659f569b743abf051e701f3a2c9f1b |

- `uv run pytest app/tests/test_schema_contracts.py`：197 passed；`uv run ruff check app`：All checks passed。

## 边界

- 测试夹具通过 `file://` 传输走真实硬编码克隆，仅限测试层：`cloneGit` 的 `protocol.file.allow`/`GIT_ALLOW_PROTOCOL` 参数是测试缝，生产入口 `fetchGitCLI` 恒为 https-only 且 file 传输 "never"。服务端测试无法注入未导出的 `gitFetch` 缝，HTTP 层仅覆盖失败路径（与 remote 导入一致），正向 201 流程在 store 层覆盖。
- DNS 解绑残余风险：URL 校验在请求时解析主机名，克隆时的实际连接未做 IP 钉扎；主机名→IP 漂移未被本轮防御。
- 未在真实 git 托管服务（GitHub/GitLab 等）联测；未做 OS 实机验证。本机 git 2.43.0 通过 `LookPath` 检测，缺失时返回 503 skill_import_unavailable。
- 克隆预算 50 秒（60 秒请求上下文内）；浅克隆 `--depth 1`，不获取历史。
