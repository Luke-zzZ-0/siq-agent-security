import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import tempfile

binary = Path('/tmp/siq-ornith-fix-builds/agentshield-linux-arm64')
rows = []
marker = dict(schema='state-format/v1', program_version='fixture', format_version=1, published_at='2026-09-13T09:00:00Z')


def tree(root):
    result = {}
    for p in sorted(root.rglob('*')):
        mode = p.lstat().st_mode
        data = os.readlink(p) if stat.S_ISLNK(mode) else hashlib.sha256(p.read_bytes()).hexdigest() if stat.S_ISREG(mode) else ''
        result[str(p.relative_to(root))] = [mode, data]
    return result


def run(root, args):
    env = {k: v for k, v in os.environ.items() if not k.startswith(('SIQ_AGENT_SECURITY_', 'AGENTSHIELD_'))}
    env['SIQ_AGENT_SECURITY_STATE_DIR'] = str(root)
    return subprocess.run([str(binary), *args], env=env, capture_output=True, text=True, timeout=10)


with tempfile.TemporaryDirectory(prefix='siq-format-native-') as base:
    base = Path(base)
    for kind in ['future', 'old', 'corrupt', 'duplicate', 'trailing', 'symlink', 'unknown']:
        root = base / kind
        root.mkdir(mode=0o700)
        m = dict(marker)
        m['format_version'] = 999 if kind == 'future' else 0 if kind == 'old' else 1
        raw = json.dumps(m)
        if kind == 'corrupt':
            raw = 'not json'
        if kind == 'duplicate':
            raw = raw.replace('"format_version": 1', '"format_version": 999, "format_version": 1')
        if kind == 'trailing':
            raw += ' {}'
        if kind == 'symlink':
            target = base / 'outside'
            target.write_text('unchanged')
            (root / 'state-format.json').symlink_to(target)
        elif kind == 'unknown':
            (root / 'unknown-data').write_text('preserve')
        else:
            (root / 'state-format.json').write_text(raw)
        before = tree(base)
        for command in [['init'], ['pubkey'], ['serve', '--state-dir', str(root)]]:
            result = run(root, command)
            assert result.returncode != 0 and 'incompatible state directory' in result.stderr, (kind, command[0], result.returncode)
            assert tree(base) == before, (kind, command[0], 'mutation')
            rows.append(dict(case=kind, command=command[0], exit_code=result.returncode, rejected=True, zero_writes=True))
    root = base / 'fresh'
    assert run(root, ['init']).returncode == 0
    # pubkey historically generates the signing identity lazily; prepare it
    # before asserting a later read is read-only. Never log the returned key.
    assert run(root, ['pubkey']).returncode == 0
    marked = (root / 'state-format.json').read_bytes()
    before = tree(root)
    assert run(root, ['init']).returncode == 0 and tree(root) == before
    rows.append(dict(case='fresh-init-repeat', marker_version=json.loads(marked)['format_version'], idempotent=True))
    (root / 'state-format.json').unlink()
    original = tree(root)
    assert run(root, ['pubkey']).returncode == 0 and tree(root) == original
    rows.append(dict(case='known-unmarked-read', zero_writes=True, no_automatic_marker=True))
    assert run(root, ['init']).returncode == 0
    after = tree(root)
    after.pop('state-format.json')
    assert after == original
    rows.append(dict(case='explicit-unmarked-init', only_added_marker=True, historical_bytes_preserved=True))
Path('/tmp/siq-ornith-fix-native-format.json').write_text(json.dumps(dict(platform='linux/arm64', binary_sha256=hashlib.sha256(binary.read_bytes()).hexdigest(), cases=rows), indent=2) + '\n')
print('Native format scenarios passed:', len(rows))
