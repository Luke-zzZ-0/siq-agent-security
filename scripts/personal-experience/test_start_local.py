"""Lifecycle regression tests use a fake executable only as a process fixture."""

import importlib.util
import json
import os
import socket
import subprocess
import sys
import time
from pathlib import Path

import pytest

spec = importlib.util.spec_from_file_location("personal_start_local", Path(__file__).with_name("start-local.py"))
launcher = importlib.util.module_from_spec(spec)
spec.loader.exec_module(launcher)


@pytest.fixture
def native_start_timeout():
    timeout = float(os.environ.get("SIQ_TEST_START_TIMEOUT", "15"))
    assert 0 < timeout < float("inf")
    return timeout


@pytest.mark.skipif(not os.environ.get("SIQ_TEST_BINARY"), reason="requires an explicitly built native binary")
def test_native_start_command(tmp_path, native_start_timeout):
    binary = Path(os.environ["SIQ_TEST_BINARY"]).resolve(strict=True)
    selected = tmp_path / "fresh"
    env = {**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(selected)}
    with socket.socket() as reserve:
        reserve.bind(("127.0.0.1", 0))
        port = reserve.getsockname()[1]
    child = subprocess.Popen(
        [str(binary), "start", "--port", str(port)], env=env,
        stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    try:
        deadline = time.monotonic() + native_start_timeout
        while not launcher.healthy(binary, port, env):
            assert child.poll() is None, "native start exited before readiness"
            assert time.monotonic() < deadline, "native start did not become ready"
            time.sleep(0.1)
        initial = (selected / "local-instance.json").read_bytes()
        reused = subprocess.run(
            [str(binary), "start"], env=env, capture_output=True, text=True, encoding="utf-8", timeout=10, check=True,
        )
        assert json.loads(reused.stdout)["schema_version"] == "local-service-instance-health/v1"
        other = tmp_path / "other"
        refused = subprocess.run(
            [str(binary), "start", "--port", str(port)],
            env={**env, "SIQ_AGENT_SECURITY_STATE_DIR": str(other)},
            capture_output=True, timeout=10, check=False,
        )
        assert refused.returncode != 0
        assert not other.exists()
        assert child.poll() is None
        assert (selected / "local-instance.json").read_bytes() == initial
        subprocess.run(
            [str(binary), "stop", "--confirm-stop"], env=env,
            capture_output=True, timeout=45, check=True,
        )
        child.wait(timeout=10)
    finally:
        if child.poll() is None:
            child.terminate()
        try:
            child.wait(timeout=10)
        except subprocess.TimeoutExpired:
            child.kill()
            child.wait(timeout=10)
    if os.name != "nt":
        assert not (selected / "serve.lock").exists()
    assert (selected / "local-instance.json").read_bytes() == initial


def test_repeated_start_reuses_verified_service_without_spawning(monkeypatch, tmp_path):
    monkeypatch.setattr(launcher, "healthy", lambda *args: True)
    monkeypatch.setattr(subprocess, "Popen", lambda *args, **kwargs: pytest.fail("reused service spawned a child"))
    result = launcher.ensure_started(tmp_path / "binary", tmp_path / "state", 47611)
    assert result["reused"] is True
    assert not (tmp_path / "state").exists()


def test_unknown_port_owner_is_not_started_or_killed(monkeypatch, tmp_path):
    monkeypatch.setattr(launcher, "healthy", lambda *args: False)
    monkeypatch.setattr(subprocess, "Popen", lambda *args, **kwargs: pytest.fail("occupied port spawned a child"))
    with socket.socket() as listener:
        listener.bind(("127.0.0.1", 0))
        listener.listen()
        port = listener.getsockname()[1]
        with pytest.raises(RuntimeError, match="占用"):
            launcher.ensure_started(tmp_path / "binary", tmp_path / "state", port)
        assert listener.fileno() >= 0
        assert not (tmp_path / "state").exists()


@pytest.mark.parametrize("network_error", [TimeoutError(), PermissionError(), OSError("unknown network error")])
def test_unconfirmed_port_never_initializes_or_starts(monkeypatch, tmp_path, network_error):
    monkeypatch.setattr(launcher, "healthy", lambda *args: False)

    def cannot_confirm(*args, **kwargs):
        raise network_error

    monkeypatch.setattr(socket, "create_connection", cannot_confirm)
    monkeypatch.setattr(launcher, "initialize", lambda *args: pytest.fail("unconfirmed port initialized state"))
    monkeypatch.setattr(subprocess, "Popen", lambda *args, **kwargs: pytest.fail("unconfirmed port spawned a child"))
    state = tmp_path / "state"
    with pytest.raises(RuntimeError, match="无法确认本地端口可用"):
        launcher.ensure_started(tmp_path / "binary", state, 47611)
    assert not state.exists()


@pytest.mark.skipif(
    os.name == "nt", reason="fixture uses Unix executable shebang; Windows runtime evidence tracked separately"
)
def test_spawn_failure_preserves_writer_lock(monkeypatch, tmp_path):
    monkeypatch.setattr(launcher, "initialize", lambda *args: None)
    binary = tmp_path / "failed-binary"
    binary.write_text(f"#!{sys.executable}\nraise SystemExit(1)\n")
    binary.chmod(0o700)
    state = tmp_path / "state"
    state.mkdir()
    lock = state / "writer.lock"
    lock.write_text("owned-lock-fixture")
    with socket.socket() as reserve:
        reserve.bind(("127.0.0.1", 0))
        port = reserve.getsockname()[1]
    with pytest.raises(RuntimeError, match="启动失败"):
        launcher.ensure_started(binary, state, port)
    assert lock.read_text() == "owned-lock-fixture"


def test_health_requires_protocol_not_exit_code(monkeypatch, tmp_path):
    monkeypatch.setattr(
        subprocess, "run", lambda *args, **kwargs: subprocess.CompletedProcess([], 0, "<html>other app</html>")
    )
    assert not launcher.healthy(tmp_path / "binary", 47611, {})
    monkeypatch.setattr(
        subprocess,
        "run",
        lambda *args, **kwargs: subprocess.CompletedProcess(
            [],
            0,
            json.dumps(
                {
                    "schema_version": "local-service-instance-health/v1",
                    "product": "siq-agent-security",
                    "local_mode": True,
                    "status": "ready",
                }
            ),
        ),
    )
    assert launcher.healthy(tmp_path / "binary", 47611, {})


@pytest.mark.parametrize("command", ["status", "init"])
def test_utf8_json_and_diagnostics_do_not_depend_on_console_encoding(monkeypatch, tmp_path, command):
    response = (
        {"schema_version": "local-service-instance-health/v1", "product": "siq-agent-security",
         "local_mode": True, "status": "ready", "message": "就绪"}
        if command == "status"
        else {"schema_version": "local-client-initialization/v1", "status": "initialized",
              "port": 47611, "instance_id": "a" * 64, "state_directory_id": "b" * 64, "message": "就绪"}
    )
    stdout = json.dumps(response, ensure_ascii=False).encode("utf-8")
    stderr = "本地服务：诊断信息。\n".encode()
    original = subprocess.run

    def fixture_process(*args, **kwargs):
        assert args[0][1] == command
        return original(
            [sys.executable, "-c", f"import sys; sys.stdout.buffer.write({stdout!r}); sys.stderr.buffer.write({stderr!r})"],
            **kwargs,
        )

    monkeypatch.setattr(subprocess, "run", fixture_process)
    if command == "status":
        assert launcher.healthy(tmp_path / "binary", 47611, os.environ.copy())
    else:
        launcher.initialize(tmp_path / "binary", 47611, os.environ.copy())


def test_legacy_health_is_not_reused(monkeypatch, tmp_path):
    legacy = {
        "schema_version": "local-service-health/v1",
        "product": "siq-agent-security",
        "local_mode": True,
        "status": "ready",
    }
    monkeypatch.setattr(
        subprocess, "run", lambda *args, **kwargs: subprocess.CompletedProcess([], 0, json.dumps(legacy))
    )
    assert not launcher.healthy(tmp_path / "binary", 47611, {})


@pytest.mark.skipif(not os.environ.get("SIQ_TEST_BINARY"), reason="requires an explicitly built native binary")
def test_native_instance_lifecycle(monkeypatch, tmp_path, native_start_timeout):
    binary = Path(os.environ["SIQ_TEST_BINARY"]).resolve(strict=True)
    selected = tmp_path / "selected"
    selected.mkdir()
    other = tmp_path / "other"
    other.mkdir()
    with socket.socket() as reserve:
        reserve.bind(("127.0.0.1", 0))
        port = reserve.getsockname()[1]
    children = []
    original = subprocess.Popen

    def spawn(*args, **kwargs):
        child = original(*args, **kwargs)
        if "serve" in args[0]:
            children.append(child)
        return child

    monkeypatch.setattr(subprocess, "Popen", spawn)
    try:
        first = launcher.ensure_started(binary, selected, port, timeout=native_start_timeout)
        assert not first["reused"]
        initial_record = (selected / "local-instance.json").read_bytes()
        assert launcher.ensure_started(binary, selected, port)["reused"]
        assert (selected / "local-instance.json").read_bytes() == initial_record
        busy = subprocess.run(
            [str(binary), "init"],
            env={**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(selected)},
            capture_output=True, text=True, encoding="utf-8", timeout=10, check=False,
        )
        assert busy.returncode != 0
        assert "write lock held" in busy.stderr
        assert (selected / "local-instance.json").read_bytes() == initial_record
        with pytest.raises(RuntimeError, match="不同状态目录"):
            launcher.ensure_started(binary, other, port)
        assert len(children) == 1
        assert list(other.iterdir()) == []
        if os.name != "nt":
            alias = tmp_path / "alias"
            alias.symlink_to(selected, target_is_directory=True)
            # Direct callers must respect the state store's symlink barrier.
            # The CLI resolves its explicit path before invoking this helper.
            with pytest.raises(RuntimeError, match="端口被其他服务"):
                launcher.ensure_started(binary, alias, port)
            assert alias.is_symlink()
            assert (selected / "local-instance.json").read_bytes() == initial_record
            assert len(children) == 1 and children[0].poll() is None
            assert launcher.ensure_started(binary, alias.resolve(), port)["reused"]
        for directory, success in [(other, False), (selected, True)]:
            result = subprocess.run(
                [str(binary), "pair", "--port", str(port)],
                env={**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(directory)},
                capture_output=True, text=True, encoding="utf-8", timeout=10, check=False,
            )
            assert (result.returncode == 0) is success
            if not success:
                assert "different state directory" in result.stderr
        assert children[0].poll() is None
        subprocess.run(
            [str(binary), "stop", "--confirm-stop"],
            env={**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(selected)},
            capture_output=True, timeout=45, check=True,
        )
        children[0].wait(timeout=10)
        assert not launcher.ensure_started(binary, selected, port, timeout=native_start_timeout)["reused"]
        assert (selected / "local-instance.json").read_bytes() == initial_record
        subprocess.run(
            [str(binary), "stop", "--confirm-stop"],
            env={**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(selected)},
            capture_output=True, timeout=45, check=True,
        )
        children[-1].wait(timeout=10)
    finally:
        for child in children:
            if child.poll() is None:
                child.terminate()
            try:
                child.wait(timeout=10)
            except subprocess.TimeoutExpired:
                child.kill()
                child.wait(timeout=10)


def test_failed_initialization_never_starts_service(monkeypatch, tmp_path):
    monkeypatch.setattr(launcher, "healthy", lambda *args: False)
    monkeypatch.setattr(
        subprocess, "run", lambda *args, **kwargs: subprocess.CompletedProcess([], 1, "", "fixture failure")
    )
    monkeypatch.setattr(subprocess, "Popen", lambda *args, **kwargs: pytest.fail("failed init started service"))
    with socket.socket() as reserve:
        reserve.bind(("127.0.0.1", 0))
        port = reserve.getsockname()[1]
    with pytest.raises(RuntimeError, match="初始化失败"):
        launcher.ensure_started(tmp_path / "binary", tmp_path / "state", port)


@pytest.mark.skipif(not os.environ.get("SIQ_TEST_BINARY"), reason="requires an explicitly built native binary")
def test_native_initialization_concurrency(tmp_path):
    binary = Path(os.environ["SIQ_TEST_BINARY"]).resolve(strict=True)
    selected = tmp_path / "state"
    env = {**os.environ, "SIQ_AGENT_SECURITY_STATE_DIR": str(selected)}
    children = []
    try:
        for _ in range(2):
            children.append(subprocess.Popen(
                [str(binary), "init"], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                text=True, encoding="utf-8",
            ))
        identities = []
        for child in children:
            stdout, stderr = child.communicate(timeout=15)
            if child.returncode == 0:
                identities.append(json.loads(stdout)["instance_id"])
            else:
                assert "lock" in stderr
        assert identities
        repeated = subprocess.run(
            [str(binary), "init"], env=env, capture_output=True, text=True, encoding="utf-8", timeout=15, check=True,
        )
        identity = json.loads(repeated.stdout)["instance_id"]
        assert all(item == identity for item in identities)
        assert not (selected / "serve.lock").exists()
        assert not (selected / "token").exists()
    finally:
        for child in children:
            if child.poll() is None:
                child.kill()
            child.wait(timeout=10)
