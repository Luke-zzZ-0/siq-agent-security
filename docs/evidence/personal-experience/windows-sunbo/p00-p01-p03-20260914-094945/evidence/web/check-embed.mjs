import fs from 'node:fs';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';

const prefix = 'apps/agentshield/internal/ui/embedded';
const listing = execFileSync('git', ['ls-tree', '-r', '-z', 'HEAD', '--', prefix], { encoding: 'utf8' });
const mismatches = [];
const expectedPaths = new Set();
let checked = 0;
for (const entry of listing.split('\0').filter(Boolean)) {
  const [, kind, expected, file] = /^(\d+) (\w+) ([0-9a-f]+)\t(.+)$/.exec(entry).slice(1);
  if (kind !== 'blob') throw new Error('unexpected object');
  expectedPaths.add(file);
  const content = fs.readFileSync(file);
  const actual = createHash('sha1').update(`blob ${content.length}\0`).update(content).digest('hex');
  checked++;
  if (actual !== expected) mismatches.push({ path: file, expected, actual });
}
function files(directory) {
  return fs.readdirSync(directory, { withFileTypes: true }).flatMap(entry => {
    const item = `${directory}/${entry.name}`;
    if (entry.isSymbolicLink()) throw new Error('unexpected embed symlink');
    return entry.isDirectory() ? files(item) : [item];
  });
}
const extras = files(prefix).filter(file => !expectedPaths.has(file));
const result = { candidate_sha: execFileSync('git', ['rev-parse', 'HEAD'], {encoding:'utf8'}).trim(), comparison: 'raw worktree bytes against HEAD Git blobs without line-ending filters', checked_files: checked, mismatches, extras, passed: mismatches.length === 0 && extras.length === 0 };
fs.writeFileSync('.tmp/win-task-native/formal/web-launcher/embed-byte-check.json', JSON.stringify(result, null, 2) + '\n');
console.log(JSON.stringify(result, null, 2));
process.exitCode = result.passed ? 0 : 1;
