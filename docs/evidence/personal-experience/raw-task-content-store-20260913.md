# M113 默认关闭的独立加密原文仓

日期：2026-09-13；任务：UX-013；规格：§3.12.22、ADR-048。

新增 `internal/rawcontent`。`OpenExisting` 在未启用时返回 disabled 且不创建密钥/目录；`Initialize` 仅供后续明确管理授权调用，生成独立 32 字节密钥。每条结构化内容使用 AES-256-GCM、随机 nonce、完整封套 AAD和明文 SHA-256，磁盘路径独立于 receipts、effect-evidence 和 audit，任务 ID 只保存摘要。

加密前整项移除显式 secret、常见凭据字段名以及 Bearer、私钥、`sk-`、AWS access key 等值形态。没有可保存字段、非法/重复路径、控制字符、无效类型、单条/磁盘/保留期越界均失败。读取绑定任务并核对期限、认证标签和摘要；删除及两阶段过期清理只删除验证过的密文封套。

验证通过：

- `go test ./internal/rawcontent`、定向 race 和 vet。
- 测试覆盖默认无副作用、加密往返、磁盘隐私、凭据移除、跨任务读取/删除、到期、AAD 篡改、损坏密钥、非法限制和清理不触碰模拟事实链。
- 固定 Go 密文封套由 `local-raw-task-content-envelope.v1.schema.json` 校验；Control API 合同共 175 项通过，Ruff 通过。
- `git diff --check` 通过。

限制：当前只有安全存储核心，服务启动不会初始化或调用它，所以默认仍不采集原文。任务级授权、采集点、读取/删除 API、设置界面、密钥生命周期与诊断包检查继续待办。内置凭据识别是纵深防御；后续采集调用方仍必须提供可信字段分类。本批未提交、推送或合并。
