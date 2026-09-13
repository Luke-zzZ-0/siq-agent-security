"""Managed instances require identity enrollment even in advisory modes."""

import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer

import pytest
from test_adapter import _Fake, load


def managed(mod, token):
    identity = "ri-" + "a" * 32
    agent = "hri-" + "b" * 32
    token.write_text(identity + "." + "c" * 64)
    mod._CFG.update(runtime_identity_id=identity, agent_id=agent, token_path=str(token))
    mod._TOKEN = None
    _Fake.enroll = {
        "schema_version": "local-runtime-session-enrolled/v1", "identity_id": identity,
        "platform": "hermes", "agent_id": agent, "session_id": "native-session",
        "binding_id": "bind-" + "d" * 64, "intent_id": "int-ri-" + "e" * 64,
        "expires_at": "2099-01-01T00:00:00Z",
    }
    return identity


@pytest.mark.parametrize("mode", ["block", "warn", "audit_only"])
def test_managed_enrollment_precedes_decision_and_observation(server, mode):
    srv, token = server
    mod = load(mode, f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    _Fake.decision = {"action": "allow", "reason": "ok", "receipt_id": "r", "action_id": "act-1"}
    assert mod._pre_tool_call("read_file", {}, session_id="native-session", tool_call_id="call-1") is None
    mod._post_tool_call("read_file", {}, result="ok", session_id="native-session", tool_call_id="call-1")
    assert [row[0] for row in _Fake.seen] == [
        "/v1/runtime-sessions",
        "/v1/decide",
        "/v1/raw-task-content/native-captures",
        "/v1/observe",
        "/v1/raw-task-content/native-captures",
    ]
    assert all(row[1] == "Bearer " + token.read_text() for row in _Fake.seen)
    assert _Fake.seen[0][2] == {
        "schema_version": "local-runtime-session-enroll/v1", "session_id": "native-session",
    }
    assert "c" * 64 not in str([row[2] for row in _Fake.seen])


def test_managed_native_raw_capture_is_scoped_flattened_and_best_effort(server):
    srv, token = server
    mod = load("block", f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    _Fake.decision = {"action": "allow", "reason": "ok", "receipt_id": "r", "action_id": "act-raw"}
    arguments = {
        "path": "/work/report.md",
        "api_key": "PRIVATE_API_KEY",
        "nested": {"authorization": "Bearer " + "A" * 32, "safe": "keep"},
    }
    assert mod._pre_tool_call(
        "read_file", arguments, session_id="native-session", tool_call_id="raw-call"
    ) is None
    capture = [row for row in _Fake.seen if row[0] == "/v1/raw-task-content/native-captures"][-1]
    assert capture[1] == "Bearer " + token.read_text()
    body = capture[2]
    assert set(body) == {"schema_version", "platform", "agent_id", "session_id", "kind", "fields"}
    assert body["kind"] == "parameters" and body["session_id"] == "native-session"
    fields = {field["path"]: field["value"] for field in body["fields"]}
    assert fields["/tool/arguments/path"] == '"/work/report.md"'
    assert fields["/tool/arguments/api_key"] == '"PRIVATE_API_KEY"'
    assert fields["/tool/arguments/nested/safe"] == '"keep"'
    assert "task_id" not in body and "grant_id" not in body and "permit" not in body

    _Fake.status = 503
    mod._post_tool_call(
        "read_file", arguments, result={"result": "kept", "token": "PRIVATE_TOKEN"},
        session_id="native-session", tool_call_id="raw-call",
    )
    assert _Fake.seen[-1][0] == "/v1/raw-task-content/native-captures"

    unmanaged = load("block", f"http://127.0.0.1:{srv.server_port}", token)
    _Fake.status = 200
    _Fake.seen = []
    _Fake.decision = {"action": "allow", "reason": "ok", "receipt_id": "r"}
    assert unmanaged._pre_tool_call("read_file", {}, session_id="legacy") is None
    unmanaged._post_tool_call("read_file", {}, result="ok", session_id="legacy")
    assert not any(row[0] == "/v1/raw-task-content/native-captures" for row in _Fake.seen)


def test_managed_blocked_post_result_is_not_captured(server):
    srv, token = server
    mod = load("block", f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    _Fake.decision = {"action": "deny", "reason": "blocked", "receipt_id": "r"}
    assert mod._pre_tool_call(
        "write_file", {"path": "/blocked"}, session_id="native-session", tool_call_id="blocked-call"
    )["action"] == "block"
    mod._post_tool_call(
        "write_file", {"path": "/blocked"}, result="siq-agent-security blocked",
        session_id="native-session", tool_call_id="blocked-call",
    )
    assert not any(row[0] == "/v1/raw-task-content/native-captures" for row in _Fake.seen)


@pytest.mark.parametrize("flow", ["duplicate_post", "duplicate_pre", "missing_reference"])
def test_native_output_requires_single_unused_allow_reference(server, flow):
    srv, token = server
    mod = load("block", f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    _Fake.decision = {"action": "allow", "reason": "ok", "receipt_id": "r"}
    if flow != "missing_reference":
        _Fake.decision["action_id"] = "act-1"
    kwargs = {"session_id": "native-session", "tool_call_id": "one-call"}
    assert mod._pre_tool_call("read_file", {}, **kwargs) is None
    if flow == "duplicate_pre":
        _Fake.decision = {"action": "deny", "reason": "denied", "receipt_id": "r2"}
        assert mod._pre_tool_call("read_file", {}, **kwargs)["action"] == "block"
    mod._post_tool_call("read_file", {}, result="first", **kwargs)
    mod._post_tool_call("read_file", {}, result="duplicate", **kwargs)
    outputs = [row for row in _Fake.seen if row[0] == "/v1/raw-task-content/native-captures" and row[2]["kind"] == "output"]
    assert len(outputs) == (1 if flow == "duplicate_post" else 0)


def test_native_raw_flattening_rejects_partial_or_unbounded_values(server):
    srv, token = server
    mod = load("block", f"http://127.0.0.1:{srv.server_port}", token)
    assert mod._raw_content_fields("tool", "/tool/arguments", {str(i): i for i in range(1024)}) is None
    value = {}
    cursor = value
    for index in range(34):
        cursor[str(index)] = {}
        cursor = cursor[str(index)]
    assert mod._raw_content_fields("tool", "/tool/arguments", value) is None
    assert mod._raw_content_fields("tool", "/tool/arguments", {"bad\nkey": "value"}) is None


@pytest.mark.parametrize("mode", ["block", "warn", "audit_only"])
@pytest.mark.parametrize(
    "bad", ["agent", "identity", "session", "binding", "extra", "unavailable", "no_native_session"]
)
def test_managed_enrollment_failure_blocks_all_modes(server, mode, bad):
    srv, token = server
    mod = load(mode, f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    session = "native-session"
    if bad in ("agent", "identity", "session", "binding"):
        field = {"agent": "agent_id", "identity": "identity_id", "session": "session_id", "binding": "binding_id"}[bad]
        _Fake.enroll[field] = "wrong"
    elif bad == "extra":
        _Fake.enroll["unexpected"] = True
    elif bad == "unavailable":
        _Fake.status = 401
    else:
        session = ""
    result = mod._pre_tool_call("write_file", {}, session_id=session, task_id="not-native-session")
    assert result["action"] == "block"
    assert not any(row[0] == "/v1/decide" for row in _Fake.seen)
    pending = (token.parent / "pending/decisions.jsonl").read_text()
    assert '"outcome":"deny"' in pending and token.read_text() not in pending


@pytest.mark.parametrize("mode", ["warn", "audit_only"])
def test_managed_authority_loss_after_enrollment_cannot_fail_open(server, mode, monkeypatch):
    srv, token = server
    mod = load(mode, f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    original = mod._post
    monkeypatch.setattr(
        mod, "_post", lambda path, body, **kw: None if path == "/v1/decide" else original(path, body, **kw)
    )
    assert mod._pre_tool_call("read_file", {}, session_id="native-session")["action"] == "block"


def test_managed_config_locks_agent_and_corruption_blocks(server, tmp_path, monkeypatch):
    srv, token = server
    mod = load("warn", f"http://127.0.0.1:{srv.server_port}", token)
    managed(mod, token)
    monkeypatch.setattr(mod, "__file__", str(tmp_path / "__init__.py"))
    config = tmp_path / "config.json"
    config.write_text(json.dumps(mod._CFG))
    monkeypatch.setenv("SIQ_AGENT_SECURITY_AGENT_ID", "borrowed-agent")
    assert mod._load_config()["agent_id"] == "hri-" + "b" * 32
    for text in ["broken", "[]", "x" * 65537]:
        config.write_text(text)
        mod._CFG = mod._load_config()
        assert mod._pre_tool_call("read_file", {}, session_id="native-session")["action"] == "block"
    assert not _Fake.seen


def test_decision_credentials_never_follow_redirect_or_proxy(server, monkeypatch):
    srv, token = server
    seen = []

    class Target(BaseHTTPRequestHandler):
        def do_GET(self):
            seen.append(self.headers.get("Authorization"))
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{}')

        do_POST = do_GET

        def log_message(self, *_):
            pass

    target = HTTPServer(("127.0.0.1", 0), Target)
    thread = threading.Thread(target=target.serve_forever, daemon=True)
    thread.start()

    class Redirect(Target):
        def do_POST(self):
            self.send_response(302)
            self.send_header("Location", f"http://127.0.0.1:{target.server_port}/destination")
            self.end_headers()

    redirect = HTTPServer(("127.0.0.1", 0), Redirect)
    redirect_thread = threading.Thread(target=redirect.serve_forever, daemon=True)
    redirect_thread.start()
    try:
        mod = load("block", f"http://127.0.0.1:{redirect.server_port}", token)
        assert mod._post("/v1/decide", {}) is None
        assert seen == []
        monkeypatch.setenv("http_proxy", f"http://127.0.0.1:{target.server_port}")
        monkeypatch.setenv("no_proxy", "")
        monkeypatch.setenv("NO_PROXY", "")
        mod._CFG["endpoint"] = f"http://127.0.0.1:{srv.server_port}"
        _Fake.decision = {"action": "allow"}
        assert mod._post("/v1/decide", {}) == {"action": "allow"}
        assert seen == [] and len(_Fake.seen) == 1
    finally:
        redirect.shutdown()
        redirect.server_close()
        target.shutdown()
        target.server_close()
        redirect_thread.join(timeout=2)
        thread.join(timeout=2)


def test_localhost_credential_transport_is_pinned(server):
    srv, token = server
    mod = load("block", f"http://localhost:{srv.server_port}", token)
    assert mod._local_endpoint() == f"http://127.0.0.1:{srv.server_port}"
    _Fake.decision = {"action": "allow"}
    assert mod._post("/v1/decide", {}) == {"action": "allow"}
    for endpoint in ["http://localhost", "http://localhost:0", "http://localhost:65536", "http://user@localhost:80"]:
        mod._CFG["endpoint"] = endpoint
        assert mod._local_endpoint() is None
