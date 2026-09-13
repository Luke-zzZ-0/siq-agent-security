# M110 完整脱敏追溯包核心

日期：2026-09-13；任务：UX-011/013；规格：§3.12.19。

新增 `internal/export/trace_document.go` 与 `local-task-trace-export/v1`。构建器先复验 M101 活动摘要签名与结构，再要求每条历史 Skill 来源和回执 `seq/source_hash` 一一对应；完成结论须属于同一任务，引用的效果记录逐份验签并拒绝缺失、重复和跨任务材料。`verified` 结论必须含至少一个全部核验且有证据的要求，并且没有事件引用，避免把不完整材料签成成功。

输出保留受控状态、时间、内容/权限/制品摘要和效果资源摘要。任务、Skill 名称/版本、Grant、准入、导入、要求、证据、动作、裁决回执、效果类型和来源标识只输出 SHA-256 引用；不复制参数、理由原文、文件或网络观察材料、原始签名和私钥。历史来源不可用或完成结论非 `verified` 时 `incomplete=true`，合同同时禁止 `incomplete=false` 与上述状态矛盾。

验证通过：

- `go test ./...`，日志 `/tmp/siq-m110-go-test.log`；`go vet ./...`。
- `go test -race ./internal/export -run TraceExport -count=1`。
- Go 隐私/关系负向覆盖敏感原文、缺失证据、重复证据、跨任务、错误来源绑定、虚假 verified、明确 unavailable 状态、篡改和错误验签公钥。
- 固定 Go 样例 `apps/agentshield/testdata/contracts/local-task-trace-export.json` 不含测试原始任务、证据、动作、回执、观察者或私密 Skill 标识。
- Control API 的 Draft 7 合同回归与 Ruff 通过；Python 使用独立固定公钥验证 Go 生成的 Ed25519 签名，并验证投影篡改失败。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 go build -trimpath ./cmd/agentshield` 通过。
- `git diff --check` 通过。

限制：这是本地核心和跨语言固定样例，还没有服务端原子快照编排、下载 API 或前端入口。`redacted_trace_projection_only` 只证明本机签名的脱敏投影，内嵌公钥不是信任锚；它不证明完整历史、当前权限、实际 Skill 执行或任务成功。交叉编译也不作为 macOS/Windows 原生验收。本批未提交、推送或合并。
