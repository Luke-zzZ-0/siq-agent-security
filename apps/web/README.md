# Web 控制台开发与构建

在 `apps/web` 目录使用仓库锁文件安装依赖。Windows PowerShell 可显式使用
`npm.cmd`，无需修改 ExecutionPolicy：

```powershell
npm.cmd ci
npm.cmd test
npm.cmd run build
npm.cmd run build:local
```

企业控制台使用 `npm run dev` / `npm run build`。本地 AgentShield 控制台使用
`npm run dev:local` / `npm run build:local`；这两个本地入口通过 Node 设置
当前进程的 `VITE_APP=agentshield`，并运行锁定的本地 Vite CLI，兼容 Windows
和 POSIX shell，不需要全局 Vite 或额外依赖。两种构建均先运行 TypeScript 检查。

`dev:local` 默认监听 `127.0.0.1` 并打开 `/overview`，`/` 和 `/demo` 等页面
使用本地应用入口。附加 Vite 参数可放在 `--` 后，如
`npm.cmd run dev:local -- --port 5174 --strictPort`。

企业构建输出到 `apps/web/dist/`。本地构建直接更新
`apps/agentshield/internal/ui/embedded/`，供 Go 二进制嵌入；应先核对这些资产的
Git 差异，再固定源码候选并构建 Go。本地开发服务通过不代表交付二进制或真实
宿主已验收。
