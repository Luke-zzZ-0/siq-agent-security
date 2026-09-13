# M104 历史 Skill 内容来源核验核心

日期：2026-09-12；UX-011；规格 §3.12.13。

新增 internal/intent/historical_skill.go 与测试。复用 M103 历史选择、admission.Verify 和 importsource.Parse，按签名选择的 AdmissionID 读取准入。返回元数据副本，不读取当前安装文件，不查询当前 Grant，也不运行 Skill 内容。

普通准入 content_hash 与导入制品 artifact_digest/分析 analysis_sha256 分开保留；版本是声明字符串，缺失为 nil。导入源必须与派生准入 ID 和规范编码一致。名称/版本控制字符及限长检查用于后续展示。

验证：

- TestHistoricalSkillSource：普通/导入两种签名样例；正确 ID 唯一读取、摘要范围区别、版本副本不改原对象。
- 签名篡改、另一个合法签名准入、控制字符名称、非法签名导入来源、缺失读取函数/记录均拒绝。
- 缺失声明版本保持未知，不编造版本。
- go test ./...，日志 /tmp/siq-m104-go-test.log；go vet ./...。
- go test -race ./internal/intent -run TestHistorical。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全包 go build ./...。
- git diff --check。

测试中的签名材料是隔离本地构造，不能作为真实 Skill 执行或原生平台验收。本批仅内部核心，磁盘限额读取与查询合同/API/UI 待接；不新增外部合同。历史分析内容来源不证明实际执行版本、当前安装完整性或当前授权有效。未提交/推送。
