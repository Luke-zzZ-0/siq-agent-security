import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import tempfile
import time

REPO = Path.cwd().resolve()
OUT = REPO / ".tmp/win-task-native/formal/web-launcher"
CANDIDATE = "ebc472f2e46aa7de837afe9d6a0ed422eef51cd0"
BINARY = REPO / ".tmp/win-task-native/build/siq-candidate-windows-amd64.exe"
BINARY_SHA = "4bc3f5ae95fd00aab528363d5d91e64145e0fd75257904a9f898323de68b6f74"

def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()

def git(*args):
    return subprocess.check_output(["git", *args], cwd=REPO).decode("utf-8").strip()

def redact(text):
    for value, replacement in [(str(REPO), "<WORKSPACE>"), (str(REPO).replace("\\", "/"), "<WORKSPACE>"),
                               (os.environ.get("USERPROFILE", ""), "<USERPROFILE>")]:
        if value:
            text = text.replace(value, replacement).replace(value.replace("\\", "/"), replacement)
    return text

def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=["web", "launcher"])
    mode = parser.parse_args().mode
    assert git("rev-parse", "HEAD") == CANDIDATE
    assert git("status", "--porcelain=v1") == "", "candidate worktree changed"
    assert sha(BINARY) == BINARY_SHA
    private = Path(tempfile.mkdtemp(prefix=f"private-{mode}-", dir=REPO / ".tmp/win-task-native"))
    env = dict(os.environ, TEMP=str(private), TMP=str(private), PYTHONUTF8="1", SIQ_TEST_START_TIMEOUT="60",
               SIQ_TEST_BINARY=str(BINARY))
    env.pop("VITE_APP", None)
    report = {"candidate_sha": CANDIDATE, "source_dirty_before": False, "binary_sha256": BINARY_SHA,
              "runner_sha256": sha(Path(__file__)), "started_at": datetime.now(timezone.utc).isoformat(),
              "os": platform.system(), "architecture": platform.machine(), "python": sys.version,
              "mode": mode, "commands": [], "source_files": {}}
    files = ["apps/web/package.json", "apps/web/package-lock.json", "apps/web/scripts/vite-local.mjs"] if mode == "web" else [
        "scripts/personal-experience/start-local.py", "scripts/personal-experience/test_start_local.py"]
    report["source_files"] = {name: sha(REPO / name) for name in files}

    def save():
        (OUT / f"{mode}-verification.json").write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")

    def run(label, command, cwd=REPO, timeout=600):
        start = time.monotonic()
        result = subprocess.run(command, cwd=cwd, env=env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                                timeout=timeout, check=False, creationflags=subprocess.CREATE_NO_WINDOW)
        content = redact(result.stdout.decode("utf-8", errors="replace"))
        log = OUT / f"{label}.log"
        log.write_text(content, encoding="utf-8")
        report["commands"].append({"label": label, "argv": [redact(str(item)) for item in command],
                                   "cwd": redact(str(cwd)), "exit_code": result.returncode,
                                   "elapsed_seconds": round(time.monotonic() - start, 3),
                                   "log": log.name, "log_sha256": sha(log)})
        save()
        print(f"{label}: exit={result.returncode}, elapsed={report['commands'][-1]['elapsed_seconds']}s", flush=True)
        assert result.returncode == 0, f"{label} failed; see {log.name}"
        assert git("rev-parse", "HEAD") == CANDIDATE, "candidate SHA changed during validation"
        assert git("status", "--porcelain=v1") == "", "build changed tracked files; stop for review"

    try:
        if mode == "web":
            web = REPO / "apps/web"
            npm = shutil.which("npm.cmd")
            node = shutil.which("node.exe")
            assert npm and node
            run("node-version", [node, "--version"])
            run("npm-version", [npm, "--version"])
            run("npm-ci", [npm, "ci"], web)
            run("web-tests", [npm, "test"], web)
            run("build-enterprise", [npm, "run", "build"], web)
            run("build-local", [npm, "run", "build:local"], web)
            run("dev-local-help", [npm, "run", "dev:local", "--", "--help"], web)
            run("dev-http-entries", [node, str(OUT / "check-dev.mjs")])
            run("embed-byte-check", [node, str(OUT / "check-embed.mjs")])
        else:
            run("launcher-native-tests", [str(REPO / "apps/control-api/.venv/Scripts/python.exe"), "-m", "pytest",
                "--noconftest", "scripts/personal-experience/test_start_local.py", "-v", "-rs",
                "--basetemp", str(private / "pytest")], timeout=600)
        report["status"] = "pass"
    except BaseException as exc:
        report["status"] = "fail"
        report["error_type"] = type(exc).__name__
        report["error"] = redact(str(exc))
        raise
    finally:
        report["finished_at"] = datetime.now(timezone.utc).isoformat()
        report["source_dirty_after"] = bool(git("status", "--porcelain=v1"))
        report["candidate_after"] = git("rev-parse", "HEAD")
        report["binary_sha256_after"] = sha(BINARY)
        save()

if __name__ == "__main__":
    main()
