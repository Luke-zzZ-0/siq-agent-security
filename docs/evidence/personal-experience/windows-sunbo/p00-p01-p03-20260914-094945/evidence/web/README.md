# Windows Web 与 launcher 正式复验

候选：`ebc472f2e46aa7de837afe9d6a0ed422eef51cd0`。二进制 SHA-256：`4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74`。测试前后工作树均干净，重建未改变任何 tracked 文件。

- npm ci、企业构建、本地构建、dev:local --help：exit 0。
- Web：22 个测试文件、74 项测试通过。
- 企业/本地模式的 /、/demo、/overview 共 6 个 HTTP 入口均 200，选中正确 HTML 入口；两个自启开发进程已停止且端口关闭。未测浏览器渲染。
- 214 个 Go embed 文件与候选 Git blob 原始字节一致，无额外文件。
- launcher：13 passed、1 skipped，exit 0。唯一 skip 为原有 Unix shebang fixture；3 个实际 Windows 二进制 lifecycle/init 测试函数通过。
- 环境：PYTHONUTF8=1、SIQ_TEST_START_TIMEOUT=60；TEMP/TMP 与 pytest --basetemp 使用 .tmp/win-task-native 下新建隔离目录。

完整命令、退出码、时间与哈希见 web-verification.json、launcher-verification.json，汇总及探针哈希见 verification.json。日志仅作用户路径脱敏。该批结果不代表三个 Agent 平台完整验收。
