#!/usr/bin/env python3
"""Exercise state compatibility refusal with a native Windows binary.

The caller supplies an existing private test root with verified Windows ACLs.
Only a newly created child is used. No production state, migration backup,
credential, or raw command output is exported. This is a boundary probe, not
evidence of a supported old-to-new binary migration or power-loss durability.
"""

import argparse
import hashlib
import json
import os
import platform
import subprocess
import tempfile
from pathlib import Path


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def snapshot(root):
    items = []
    for path in sorted(root.rglob("*")):
        info = path.lstat()
        if path.is_symlink() or getattr(info, "st_file_attributes", 0) & 0x400:
            raise RuntimeError("unexpected reparse point in private fixture")
        items.append((path.relative_to(root).as_posix(), info.st_mode,
                      digest(path.read_bytes()) if path.is_file() else None))
    return digest(json.dumps(items, ensure_ascii=True).encode()), len(items)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, required=True)
    parser.add_argument("--test-root", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    if os.name != "nt" or not args.test_root.is_dir() or args.out.exists():
        parser.error("requires Windows, an existing private root, and a new output")
    binary = args.binary.resolve(strict=True)
    root = Path(tempfile.mkdtemp(prefix="state-native-", dir=args.test_root))

    def run(state, *command):
        env = {**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(state),
               "AGENTSHIELD_STATE_DIR": str(state)}
        return subprocess.run([str(binary), *command], env=env, capture_output=True,
                              text=True, encoding="utf-8", timeout=30,
                              creationflags=subprocess.CREATE_NO_WINDOW, check=False)

    checks = {}
    for name in ("future_reader", "future_writer", "duplicate_key", "wrong_instance",
                 "wrong_directory", "nested_future_parent"):
        state = root / name / "中文 空格状态"
        initialized = run(state, "init", "--port", "47619")
        if initialized.returncode:
            raise RuntimeError("fixture initialization failed")
        marker_path = state / "state-format.json"
        marker = json.loads(marker_path.read_text(encoding="utf-8"))
        if name in ("future_reader", "nested_future_parent"):
            marker.update(min_reader=99, min_writer=99)
        elif name == "future_writer":
            marker["min_writer"] = 99
        elif name == "wrong_instance":
            marker["instance_id"] = "0" * 64
        elif name == "wrong_directory":
            marker["state_directory_id"] = "0" * 64
        raw = json.dumps(marker)
        if name == "duplicate_key":
            raw = raw[:-1] + ',"format_version":2}'
        if name == "nested_future_parent":
            # A valid inner marker must never hide a future-format ancestor.
            marker_path = state.parent / "state-format.json"
            raw = json.dumps({"schema": "state-format/v1", "format_version": 99,
                              "program_version": "synthetic-future",
                              "published_at": marker["published_at"]})
        marker_path.write_text(raw, encoding="utf-8")
        before, entries = snapshot(state.parent)
        status = run(state, "state-status")
        status_data = json.loads(status.stdout) if status.returncode == 0 else {}
        init = run(state, "init", "--port", "47619")
        pubkey = run(state, "pubkey")
        after, _ = snapshot(state.parent)
        init_denied = init.returncode != 0 and "incompatible state" in init.stderr
        pubkey_denied = pubkey.returncode != 0 and "incompatible state" in pubkey.stderr
        # A future writer may remain readable; only writer refusal is required.
        reader_required = name != "future_writer"
        ok = init_denied and before == after and (pubkey_denied or not reader_required)
        if reader_required:
            ok = ok and status_data.get("compatible") is False
        checks[name] = {"status": "pass" if ok else "fail", "method": "native_cli",
                        "state_status_exit": status.returncode,
                        "state_status_compatible": status_data.get("compatible"),
                        "init_exit": init.returncode, "pubkey_exit": pubkey.returncode,
                        "writer_refused_as_incompatible": init_denied,
                        "reader_refused_as_incompatible": pubkey_denied,
                        "business_tree_unchanged": before == after,
                        "before_tree_sha256": before, "after_tree_sha256": after,
                        "entries": entries}
    result = {"schema_version": "windows-state-native-smoke/v1",
              "os": "windows", "architecture": platform.machine(),
              "binary_sha256": digest(binary.read_bytes()), "checks": checks,
              "scope": "isolated native compatibility refusal; synthetic invalid markers",
              "limits": ["not old-to-new migration", "not ACL preservation certification",
                         "not power-loss recovery", "private fixtures retained for inspection"]}
    with args.out.open("x", encoding="utf-8") as stream:
        json.dump(result, stream, ensure_ascii=False, indent=2)
        stream.write("\n")
    print(json.dumps({name: value["status"] for name, value in checks.items()}))
    return 0 if all(value["status"] == "pass" for value in checks.values()) else 1


if __name__ == "__main__":
    raise SystemExit(main())
