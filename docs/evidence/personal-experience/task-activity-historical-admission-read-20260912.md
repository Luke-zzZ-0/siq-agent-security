# M105 历史准入受限磁盘读取

日期：2026-09-12；UX-011；规格 §3.12.14。

新增 internal/state/historical_admission.go 与测试。GetHistoricalAdmission 与既有 GetAdmission 分开，复用 readCommitFile 的普通文件检查、打开后 SameFile 及 8 MiB 限额；读取前后核对 admissions 目录身份、拒绝目录链接。JSON 禁止未知字段/尾随值，ID 必须与请求一致。返回固定错误类别，缺失保留 os.ErrNotExist；不修改状态。

验证通过：

- go test ./internal/state -run TestHistoricalAdmission -count=1。
- 正常读取、恰好 8 MiB JSON（空格填充）、8 MiB + 1 拒绝。
- 文档 ID 错配、路径逃逸、空 ID、未知字段、尾随对象、null、损坏 JSON、目录对象及缺失文件。
- Linux 临时目录中的文件/父目录 symlink 拒绝；这些用例不计为 Windows 原生通过。
- go test ./...：/tmp/siq-m105-go-test.log；go vet ./...。
- go test -race ./internal/state -run TestHistoricalAdmission。
- linux/amd64、linux/arm64、darwin/arm64、windows/amd64 全包 go build ./...。
- git diff --check。

读取成功不代表验签，调用方仍须使用 M103/M104 核心验证历史选择与准入签名。对可写相同用户状态目录的恶意进程不宣称隔离保证。查询 API/UI 未在本批接通，未提交推送；无新原生平台或真实智能体验收证据。
