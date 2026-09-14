import { spawn } from 'node:child_process';
import { once } from 'node:events';
import fs from 'node:fs';
import net from 'node:net';
import path from 'node:path';

const root = process.cwd();
const web = path.join(root, 'apps/web');
const out = path.join(root, '.tmp/win-task-native/formal/web-launcher');
const results = [];
for (const local of [false, true]) {
  const listener = net.createServer();
  listener.listen(0, '127.0.0.1');
  await once(listener, 'listening');
  const port = listener.address().port;
  await new Promise((resolve) => listener.close(resolve));
  const args = [local ? 'scripts/vite-local.mjs' : 'node_modules/vite/bin/vite.js', '--host', '127.0.0.1', '--port', String(port), '--strictPort'];
  if (local) args.push('--open', '/overview');
  const env = { ...process.env, BROWSER: 'none' };
  delete env.VITE_APP;
  const child = spawn(process.execPath, args, { cwd: web, env, windowsHide: true });
  const mode = local ? 'local' : 'enterprise';
  let logs = '';
  child.stdout.on('data', (x) => { logs += x; });
  child.stderr.on('data', (x) => { logs += x; });
  const closed = once(child, 'close');
  try {
    let ready = false;
    for (let i = 0; i < 80; i++) {
      if (child.exitCode !== null) throw new Error(`${mode} server exited ${child.exitCode}`);
      try {
        const response = await fetch(`http://127.0.0.1:${port}/`, { signal: AbortSignal.timeout(1000) });
        if (response.ok) { ready = true; break; }
      } catch {}
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    if (!ready) throw new Error(`${mode} server did not become ready`);
    for (const route of ['/', '/demo', '/overview']) {
      const response = await fetch(`http://127.0.0.1:${port}${route}`);
      const html = await response.text();
      const expected = local ? '/src/local/main.tsx' : '/src/main.tsx';
      const passed = response.status === 200 && html.includes(expected) && (local ? !html.includes('src="/src/main.tsx"') : !html.includes('/src/local/main.tsx'));
      results.push({ mode, route, status: response.status, expected_entry: expected, passed });
      if (!passed) throw new Error(`${mode} ${route}: wrong HTML entry`);
    }
  } finally {
    child.kill();
    await closed;
    logs = logs.replace(/\x1b\[[0-9;]*m/g, '');
    for (const privatePath of [process.env.USERPROFILE, process.env.USERPROFILE?.replaceAll('\\', '/')]) {
      if (privatePath) logs = logs.replaceAll(privatePath, '<USERPROFILE>');
    }
    fs.writeFileSync(path.join(out, `dev-${mode}.log`), logs);
    let stopped = false;
    try { await fetch(`http://127.0.0.1:${port}/`, { signal: AbortSignal.timeout(1000) }); } catch { stopped = true; }
    results.push({ mode, cleanup: 'started process terminated', port_closed: stopped });
    fs.writeFileSync(path.join(out, 'dev-routes.json'), JSON.stringify({ method: 'native_cli', browser_rendering_tested: false, results }, null, 2) + '\n');
    if (!stopped) throw new Error(`${mode} server port remains open`);
  }
}
console.log(JSON.stringify(results, null, 2));
