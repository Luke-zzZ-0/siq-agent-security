#!/usr/bin/env python3
"""Exercise installed Windows Hermes with an isolated profile and synthetic model.

Raw state remains under an explicit private root. The output is a sanitized
development report; this does not claim desktop, approval, or Skill attribution
acceptance. A sitecustomize audit guard confines this Python fixture, not users.
"""

from __future__ import annotations

import argparse
import hashlib
import importlib.util
import json
import os
import platform
import re
import shutil
import socket
import subprocess
import sys
import time
import traceback
import urllib.error
from datetime import UTC, datetime, timedelta
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
spec = importlib.util.spec_from_file_location(
    "siq_hermes_cli_fixture", REPO / "scripts/personal-experience/hermes-cli-runtime-smoke.py"
)
cli_fixture = importlib.util.module_from_spec(spec)
spec.loader.exec_module(cli_fixture)
fixture = cli_fixture.fixture

GUARD = r'''import json, os, sys
from pathlib import Path
from urllib.parse import urlsplit

root = Path(os.environ["SIQ_FIXTURE_PRIVATE_ROOT"]).resolve()
home = Path(os.environ["SIQ_FIXTURE_REAL_HERMES_HOME"]).resolve()
code = Path(os.environ["SIQ_FIXTURE_HERMES_CODE"]).resolve()
allowed = set()
for key in ("SIQ_AGENT_SECURITY_ENDPOINT", "CUSTOM_BASE_URL"):
    value = os.environ.get(key)
    if value:
        parsed = urlsplit(value)
        allowed.add((parsed.hostname, parsed.port))

def within(path, parent):
    try:
        return path.is_relative_to(parent)
    except (ValueError, OSError):
        return False

def guard(event, args):
    if event == "socket.connect":
        address = args[1]
        if not isinstance(address, tuple) or address[:2] not in allowed:
            raise PermissionError("fixture rejects non-fixture network")
    elif event == "socket.getaddrinfo":
        if args[0] not in ("127.0.0.1", "localhost"):
            raise PermissionError("fixture rejects external DNS")
    elif event in ("subprocess.Popen", "os.system", "os.exec", "os.spawn"):
        raise PermissionError("fixture rejects host subprocess")
    elif event in ("winreg.SetValue", "winreg.SetValueEx", "winreg.DeleteKey", "winreg.DeleteValue"):
        raise PermissionError("fixture rejects registry writes")
    elif event == "open" and isinstance(args[0], (str, bytes)):
        path = Path(os.fsdecode(args[0])).resolve(strict=False)
        mode = args[1] or ""
        flags = args[2] or 0
        writing = any(char in mode for char in "wax+") or bool(
            flags & (os.O_WRONLY | os.O_RDWR | os.O_CREAT | os.O_TRUNC | os.O_APPEND)
        )
        if writing and not within(path, root):
            raise PermissionError("fixture rejects external write")
        if not within(path, root) and (
            path.name in (".env", ".op.env", "auth.json", "credentials.json", "config.yaml")
            or (within(path, home) and not within(path, code))
        ):
            raise PermissionError("fixture rejects production configuration")
    elif event in ("os.remove", "os.rmdir", "os.mkdir", "os.rename", "os.link", "os.symlink", "shutil.copyfile"):
        paths = args[:2] if event in ("os.rename", "os.link", "os.symlink", "shutil.copyfile") else args[:1]
        for raw in paths:
            if isinstance(raw, (str, bytes)) and not within(Path(os.fsdecode(raw)).resolve(strict=False), root):
                raise PermissionError("fixture rejects external mutation")

sys.addaudithook(guard)
(root / "guard-loaded.txt").write_text("python-audit-fixture-v1\n")
import faulthandler
_trace = (root / "import-stack.private.log").open("w")
faulthandler.dump_traceback_later(30, file=_trace)
'''

HOOK_WORKER = r'''import json, os, sys
from pathlib import Path
root = Path(os.environ["SIQ_FIXTURE_PRIVATE_ROOT"])
sys.path.insert(0, os.environ["SIQ_FIXTURE_HERMES_CODE"])
(root / "worker-phase.txt").write_text("import_host_plugin_dispatcher")
from hermes_cli.plugins import discover_plugins, invoke_hook, has_hook
(root / "worker-phase.txt").write_text("discover_plugins")
discover_plugins()
assert has_hook("pre_tool_call"), "pre_tool_call not loaded"
(root / "worker-phase.txt").write_text("invoke_pre_tool_call")
results = invoke_hook("pre_tool_call", tool_name="write_file",
    args={"path": str(root / "workspace/company-a/offline-must-not-exist.txt"), "content": "must not execute"},
    task_id="fixture-offline-task", session_id="fixture-offline-session", tool_call_id="offline-native-write")
(root / "worker-result.json").write_text(json.dumps(results))
(root / "worker-phase.txt").write_text("complete")
'''


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def safe_error(exc):
    return {
        "type": type(exc).__name__,
        "frames": [
            {"file": Path(item.filename).name, "line": item.lineno, "function": item.name}
            for item in traceback.extract_tb(exc.__traceback__)
        ],
    }


class WindowsHarness(cli_fixture.Harness):
    def __init__(self, root, args):
        super().__init__(root, args)
        self.binary = root / "siq-agent-security.exe"
        old_profile = Path(self.env["HERMES_HOME"])
        profile = root / "profiles" / "siq-native-fixture"
        profile.parent.mkdir()
        old_profile.rename(profile)
        home = root / "home"
        home.mkdir()
        local = home / "AppData" / "Local"
        local.mkdir(parents=True)
        roaming = home / "AppData" / "Roaming"
        roaming.mkdir(parents=True)
        temporary = root / "tmp"
        temporary.mkdir()
        guard_dir = root / "python-guard"
        guard_dir.mkdir()
        (guard_dir / "sitecustomize.py").write_text(GUARD, encoding="utf-8")
        self.env.update({
            "HOME": str(home), "USERPROFILE": str(home),
            "LOCALAPPDATA": str(local), "APPDATA": str(roaming),
            "TEMP": str(temporary), "TMP": str(temporary), "TMPDIR": str(temporary),
            "HERMES_HOME": str(profile), "PYTHONPATH": str(guard_dir),
            "PYTHONUTF8": "1", "PYTHONIOENCODING": "utf-8",
            "PYTHONDONTWRITEBYTECODE": "1", "CUSTOM_API_KEY": "siq-synthetic-fixture-key",
            "SIQ_FIXTURE_PRIVATE_ROOT": str(root),
            "SIQ_FIXTURE_REAL_HERMES_HOME": str(args.hermes_root.parent),
            "SIQ_FIXTURE_HERMES_CODE": str(args.hermes_root),
        })
        if "SYSTEMROOT" not in self.env:
            self.env["SYSTEMROOT"] = os.environ.get("SYSTEMROOT", r"C:\Windows")
        self.stages = []

    def start(self):
        # Use separate reader/writer handles: seeking an inherited stdout handle
        # is not a reliable way to poll subprocess output on Windows.
        with socket.socket() as probe:
            probe.bind(("127.0.0.1", 0))
            port = probe.getsockname()[1]
        self.endpoint = f"http://127.0.0.1:{port}"
        self.env["SIQ_AGENT_SECURITY_ENDPOINT"] = self.endpoint
        log_path = self.root / "daemon.private.log"
        self.log = log_path.open("wb")
        self.proc = subprocess.Popen(
            [str(self.binary), "serve", "--port", str(port), "--mode", "block"],
            cwd=self.workspace, env=self.env, stdout=self.log, stderr=self.log,
            creationflags=subprocess.CREATE_NO_WINDOW,
        )
        deadline = time.monotonic() + 60
        while time.monotonic() < deadline:
            fixture.require(self.proc.poll() is None, "daemon exited before readiness")
            found = re.search(rb"admin pairing code \(single use, 5 min\): (\S+)", log_path.read_bytes())
            if found:
                try:
                    pair = self.api("/v1/pair", {"code": found[1].decode("ascii")}, token="")
                    self.admin = pair["session"]
                    return
                except urllib.error.URLError:
                    pass
            time.sleep(0.1)
        raise RuntimeError("daemon readiness timeout")

    def command(self, args, *, cwd=None, timeout=120, env=None):
        label = Path(args[0]).name
        try:
            result = subprocess.run(
                args, cwd=cwd or self.workspace, env=env or self.env,
                capture_output=True, timeout=timeout, check=False,
                creationflags=subprocess.CREATE_NO_WINDOW,
            )
        except subprocess.TimeoutExpired as exc:
            self.stages.append({"executable": label, "exit_code": None, "timeout_seconds": timeout})
            index = len(self.stages)
            (self.root / f"command-{index}-stdout.private.log").write_bytes(exc.stdout or b"")
            (self.root / f"command-{index}-stderr.private.log").write_bytes(exc.stderr or b"")
            raise
        record = {"executable": label, "exit_code": result.returncode}
        self.stages.append(record)
        if result.returncode:
            # These private logs never become acceptance references directly.
            index = len(self.stages)
            (self.root / f"command-{index}-stdout.private.log").write_bytes(result.stdout)
            (self.root / f"command-{index}-stderr.private.log").write_bytes(result.stderr)
            raise RuntimeError("native command failed; private diagnostic preserved")
        return result.stdout.decode("utf-8", errors="replace")

    def path_reproduction(self):
        rows = []
        now = datetime.now(UTC)
        def stamp(value):
            return value.strftime("%Y-%m-%dT%H:%M:%SZ")
        values = [
            ("posix_absolute", "/siq-fixture/company-a", 201),
            ("windows_backslash_absolute", str(self.workspace / "company-a"), 400),
            ("windows_slash_absolute", (self.workspace / "company-a").as_posix(), 400),
            ("relative_negative", "company-a", 400),
            ("drive_relative_negative", "C:company-a", 400),
        ]
        for index, (label, value, expected) in enumerate(values):
            body = {
                "schema_version": "intent/v2", "intent_id": f"int-windows-path-{index}",
                "task_id": "task-path-reproduction", "principal": {"type": "user", "id": "fixture"},
                "agent": {"id": fixture.AGENT, "platform": "hermes"},
                "purpose": "Synthetic Windows path contract reproduction",
                "allowed_tools": ["read_file"], "allowed_effects": ["file.read"],
                "resource_constraints": [{"domain": "filesystem", "operator": "prefix", "value": value}],
                "parameter_constraints": [], "issued_at": stamp(now - timedelta(minutes=1)),
                "valid_from": stamp(now - timedelta(minutes=1)), "expires_at": stamp(now + timedelta(hours=1)),
                "authority": {"issuer": "local-admin", "revision": "r1", "evidence_ids": []},
            }
            response = self.api("/v1/intents", body, expected=expected)
            reason = response.get("reason_code", response.get("error"))
            rows.append({"case": label, "http_status": expected, "reason": reason})
        return {
            "issue": "WIN-INTENT-PATH-001", "status": "fail", "observed_failure": True,
            "method": "component_fixture", "path_values_redacted": True, "cases": rows,
            "desired_behavior": "Windows absolute resources need an explicitly defined contract and safe normalization",
            "no_filesystem_effect_executed": True,
        }

    def guard_preflight(self):
        self.command([str(self.args.hermes_python), "-c", "print('fixture interpreter ready')"])
        fixture.require((self.root / "guard-loaded.txt").exists(), "Python fixture guard not loaded")

    def offline_dispatcher(self):
        """Installed host hook dispatcher only; not the full tool dispatcher."""
        self.stop()
        forbidden = self.workspace / "company-a" / "offline-must-not-exist.txt"
        worker = self.root / "hook-worker.py"
        worker.write_text(HOOK_WORKER, encoding="utf-8")
        self.command([str(self.args.hermes_python), str(worker)], timeout=90)
        results = json.loads((self.root / "worker-result.json").read_text(encoding="utf-8"))
        fixture.require(
            any(isinstance(item, dict) and item.get("action") == "block" for item in results),
            "host hook dispatcher did not return a block directive",
        )
        text = json.dumps(results)
        fixture.require("decision service unavailable" in text, "offline denial provenance missing")
        fixture.require("fail-closed" in text, "offline fail-closed missing")
        fixture.require(not forbidden.exists(), "offline write executed")
        return {
            "status": "pass", "method": "component_fixture",
            "installed_native_hook_dispatcher": True, "full_tool_dispatcher": False, "public_cli": False,
            "block_directive_returned": True,
            "decision_service_unavailable": True, "forbidden_file_exists": False,
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hermes-cli", required=True, type=Path)
    parser.add_argument("--hermes-root", required=True, type=Path)
    parser.add_argument("--hermes-python", required=True, type=Path)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--private-root", required=True, type=Path)
    parser.add_argument("--out", required=True, type=Path)
    parser.add_argument("--mode", choices=("diagnostics", "conversation"), default="diagnostics")
    args = parser.parse_args()
    fixture.require(sys.platform == "win32", "Windows native runner required")
    for name in ("hermes_cli", "hermes_root", "hermes_python", "binary", "private_root", "out"):
        setattr(args, name, getattr(args, name).resolve())
    fixture.require(args.private_root.is_dir(), "private parent must already exist with verified ACL")
    fixture.require(not args.out.exists(), "refusing to overwrite prior report")
    fixture.require(args.out.parent.is_dir(), "report parent must already exist")
    for name in ("hermes_cli", "hermes_python", "binary"):
        fixture.require(getattr(args, name).is_file(), "required executable missing")
    for name in (".env", ".update-incomplete", ".lazy-refresh-incomplete"):
        fixture.require(not (args.hermes_root / name).exists(), "host maintenance/config marker requires review")
    source_files = {
        "hermes_cli": args.hermes_cli,
        "host_hooks": args.hermes_root / "hermes_cli/plugins.py",
        "host_dispatcher": args.hermes_root / "model_tools.py",
        "host_cli_main": args.hermes_root / "hermes_cli/main.py",
    }
    source_hashes = {name: digest(path) for name, path in source_files.items()}
    stamp = datetime.now(UTC).strftime("%Y%m%dT%H%M%S%fZ")
    root = args.private_root / f"hermes-native-{stamp}"
    root.mkdir()
    report = {
        "schema_version": "windows-hermes-native-smoke/v1",
        "recorded_at": datetime.now(UTC).isoformat(),
        "status": "fail", "source_state": "development-exploration",
        "mode": args.mode,
        "os": "windows", "architecture": platform.machine(),
        "model": "loopback-synthetic-fixture" if args.mode == "conversation" else "none",
        "paid_model_calls": False,
        "binary_sha256": digest(args.binary),
        "harness_sha256": digest(Path(__file__)), "host_source_sha256": source_hashes,
        "private_run_name": root.name,
        "limits": [
            "fixture config copied directly; not installation acceptance",
            "diagnostics uses installed native hook dispatcher only; conversation mode is a separate attempt",
            "no approval-resume, final recheck, or trusted Skill-attribution proof",
            "Python audit guard constrains fixture; not a production OS sandbox",
        ],
    }
    harness = None
    phase = "prepare"
    try:
        harness = WindowsHarness(root, args)
        shutil.copy2(args.binary, harness.binary)
        phase = "daemon_start"
        harness.start()
        phase = "fixture_guard_preflight"
        harness.guard_preflight()
        if args.mode == "conversation":
            phase = "fixture_authority"
            harness.setup_authority()
            phase = "public_cli_synthetic_conversation"
            report["public_cli"] = harness.public_cli()
        else:
            phase = "path_contract_reproduction"
            report["path_contract"] = harness.path_reproduction()
        phase = "offline_native_dispatcher"
        report["service_unavailable"] = harness.offline_dispatcher()
        report["status"] = "observed_failure" if args.mode == "diagnostics" else "pass"
    except Exception as exc:  # noqa: BLE001 -- preserve sanitized evidence for any fixture failure
        report["failed_phase"] = phase
        report["error"] = safe_error(exc)
        (root / "failure.private.json").write_text(
            json.dumps({"phase": phase, "type": type(exc).__name__, "message": str(exc)}),
            encoding="utf-8",
        )
    finally:
        if harness:
            harness.stop()
            report["command_results"] = harness.stages
        report["host_source_unchanged"] = all(
            digest(path) == source_hashes[name] for name, path in source_files.items()
        )
        if not report["host_source_unchanged"]:
            report["status"] = "fail"
        args.out.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": report["status"], "failed_phase": report.get("failed_phase"), "report": args.out.name}))
    # An observed product failure must remain a nonzero result even when the
    # independent fail-closed component probe succeeds.
    return 0 if report["status"] == "pass" else 1


if __name__ == "__main__":
    raise SystemExit(main())
