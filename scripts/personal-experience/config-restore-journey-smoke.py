#!/usr/bin/env python3
"""Full native journey on a non-default named profile, ending in restore.

Drives the complete acceptance journey from the taskbook: discovery of a
non-ASCII named Hermes profile (never the default instance the user works
in), preview, managed install with an issued runtime identity, allowed and
denied native tool calls, fail-closed blocking while the decision service is
offline, daemon restart recovery, and uninstall that restores the profile
configuration semantics with safe permissions. Uses the public Hermes CLI with a
synthetic model server; browser approval is separate evidence.
"""

import argparse
import hashlib
import importlib.util
import json
import os
import shutil
import stat
import subprocess
import tempfile
import threading
import time
from datetime import UTC, datetime
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from urllib.parse import urlsplit

REPO = Path(__file__).resolve().parents[2]
loader = importlib.util.spec_from_file_location("native_fixture", REPO / "scripts/validate-intent-v2-hermes.py")
fixture = importlib.util.module_from_spec(loader)
loader.loader.exec_module(fixture)

PROFILE_NAME = "工作 区B"
PENDING = "pending/decisions.jsonl"
CLI_TIMEOUT = 90


class Harness(fixture.Harness):
    # --- daemon lifecycle -------------------------------------------------

    def _serve(self, port):
        if port is None:
            import socket

            with socket.socket() as sock:
                sock.bind(("127.0.0.1", 0))
                port = sock.getsockname()[1]
        self.port = port
        self.endpoint = f"http://127.0.0.1:{port}"
        self.env["SIQ_AGENT_SECURITY_ENDPOINT"] = self.endpoint
        self.log = tempfile.TemporaryFile(mode="w+t")  # noqa: SIM115 -- closed in stop
        self.proc = subprocess.Popen(
            [str(self.binary), "serve", "--port", str(port), "--mode", "block"],
            cwd=self.workspace,
            env=self.env,
            stdout=self.log,
            stderr=self.log,
        )
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            fixture.require(self.proc.poll() is None, "daemon exited before readiness")
            self.log.seek(0)
            found = fixture.re.search(r"admin pairing code \(single use, 5 min\): (\S+)", self.log.read())
            if found:
                try:
                    pair = self.api("/v1/pair", {"code": found[1]}, token="")
                    self.admin = pair["session"]
                    return
                except fixture.urllib.error.URLError:
                    pass
            time.sleep(0.05)
        raise RuntimeError("daemon readiness timeout")

    def start(self):
        self._serve(None)

    def restart(self):
        """Restart the daemon on the same port; state and issued credentials persist."""
        self.stop()
        self._serve(self.port)
        self.api("/v1/adapter/instances?platform=hermes")

    # --- journey phases ---------------------------------------------------

    def create_named_profile(self):
        """A second, named, non-ASCII profile: the journey target, not the default."""
        root = Path(self.env["HERMES_HOME"]).parent.parent  # .../hermes
        profile = root / "profiles" / PROFILE_NAME
        profile.mkdir(mode=0o700)
        config = profile / "config.yaml"
        config.write_text("model: 工作区\nterminal:\n  env: local\nfixture_setting: named-profile-keep\n")
        os.chmod(config, 0o640)
        (profile / "SOUL.md").write_text("命名配置文件\n")
        self.profile = profile
        self.profile_config = config
        self.config_before = config.read_bytes()
        self.config_mode_before = stat.S_IMODE(os.stat(config).st_mode)
        self.work_config = root / "profiles/work/config.yaml"
        self.work_config_before = self.work_config.read_bytes()

    def discover(self):
        catalog = self.api("/v1/adapter/instances?platform=hermes")
        rows = catalog["instances"]
        active = [row for row in rows if row["active"]]
        fixture.require(
            len(active) == 1 and active[0]["name"] == "work",
            "default work profile not discovered as the single active instance",
        )
        target = next((row for row in rows if row["name"] == PROFILE_NAME), None)
        fixture.require(target is not None, "non-ASCII named profile was not discovered")
        fixture.require(
            target["source"] == "named_profile" and not target["active"] and not target["default"],
            "journey target must be a named, non-active profile",
        )
        fixture.require(
            not (Path(target["config_dir"]) / "plugins/siq-agent-security").exists(),
            "target profile already managed",
        )
        self.instance_id = target["instance_id"]
        self.agent = "hri-" + self.instance_id[3:]
        return {"instances_seen": len(rows), "target": PROFILE_NAME, "target_source": target["source"]}

    def setup_authority(self):
        skill = self.root / "fixture-skill"
        skill.mkdir()
        (skill / "SKILL.md").write_text(
            "---\nname: intent-fixture\ndescription: Read a synthetic report.\n"
            f"allowed-tools: {self.read_tool} {self.write_tool}\n---\nRead the fixture report.\n"
        )
        adm = self.api("/v1/admit", {"path": str(skill)})["admission"]
        fixture.require(adm["verdict"] != "quarantine", "benign fixture quarantined")
        result = self.api(
            "/v1/grants",
            {
                "admission_id": adm["admission_id"],
                "platform": self.platform,
                "subject_id": self.agent,
                "subject_type": "agent_instance",
                "redact_secrets": True,
            },
        )
        grant_path = "/v1/grants/" + result["grant"]["grant_id"]

        def action(name, **body):
            nonlocal result
            result = self.api(
                grant_path + "/" + name,
                {
                    "expected_revision": result["state_revision"],
                    "actor_id": "automated-fixture-operator",
                    **body,
                },
            )
            return result

        action(
            "patch-desired",
            tools=[self.read_tool, self.write_tool],
            filesystem={"read_only": [str(self.workspace)], "read_write": []},
        )
        for index, overlap in enumerate(result["grant"]["overlap_conflicts"]):
            if overlap["resolution"] == "unresolved":
                action("resolve-overlap", index=index)
        challenge = action("challenge")["challenge"]
        action("approve", challenge_id=challenge["challenge_id"], nonce=challenge["nonce"])
        fixture.require(result["grant"]["status"] == "approved", "grant approval did not transition")
        action("deploy")
        self.issued = self.api(
            "/v1/runtime-identities",
            {
                "schema_version": "local-runtime-identity-create/v1",
                "instance_id": self.instance_id,
                "grant_id": result["grant"]["grant_id"],
                "expected_grant_revision": result["state_revision"],
                "actor_id": "automated-fixture-operator",
                "session_ttl_seconds": 28800,
            },
            expected=201,
        )
        fixture.require(self.issued["identity"]["runtime_state"] == "unverified", "issuance claimed protection")

    def preview_and_install(self):
        profile = self.profile
        plugin_config = profile / "plugins/siq-agent-security/config.json"
        fixture.require(not plugin_config.exists(), "journey target already managed")
        before = self.profile_config.read_bytes()
        plan = self.api(
            "/v1/adapter/preview",
            {
                "platform": "hermes",
                "action": "install",
                "instance_id": self.instance_id,
                "runtime_identity_id": self.issued["identity"]["identity_id"],
                "native_enable": True,
            },
        )
        fixture.require(plan["schema_version"] == "local-adapter-plan/v3", "managed plan missing")
        fixture.require(
            self.profile_config.read_bytes() == before and not plugin_config.exists(), "preview mutated host"
        )
        self.api(
            "/v1/adapter/install",
            {
                "platform": "hermes",
                "instance_id": self.instance_id,
                "plan_id": plan["plan_id"],
                "plan_digest": plan["plan_digest"],
                "runtime_identity_id": plan["runtime_identity_id"],
                "actor_id": "automated-fixture-operator",
            },
        )
        configured = json.loads(plugin_config.read_text())
        fixture.require(
            configured["runtime_identity_id"] == self.issued["identity"]["identity_id"]
            and configured["agent_id"] == self.agent
            and configured["token_path"] == self.issued["credential_path"],
            "installed identity mismatch",
        )
        text = self.profile_config.read_text()
        fixture.require("fixture_setting: named-profile-keep" in text, "host setting lost")
        fixture.require("siq-agent-security" in text, "native enable missing")
        fixture.require(
            self.work_config.read_bytes() == self.work_config_before
            and not (self.work_config.parent / "plugins/siq-agent-security").exists(),
            "default work profile was touched by the journey install",
        )
        self.installed_files = sorted(
            str(p.relative_to(profile)) for p in profile.rglob("*") if p.is_file() and "plugins" in p.parts
        )
        return {"method": "managed_v3_preview_apply_named_profile", "changed_file_count": len(plan["changes"])}

    def run_cli(self, calls, phase, *, offline=False):
        """One public-CLI conversation driven by the synthetic model server."""
        forbidden = self.workspace / "company-a/must-not-exist.txt"
        confirmation = self.workspace / "company-a/confirmation.txt"
        confirmation.write_text("fixture-visible-company-a-confirmation\n")
        received, auxiliary, failures = [], [], []
        expect_allowed_text = not offline

        class Model(BaseHTTPRequestHandler):
            def log_message(self, *_):
                pass

            def respond(self, value):
                encoded = json.dumps(value).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(encoded)))
                self.end_headers()
                self.wfile.write(encoded)

            def do_GET(self):
                if self.path == "/v1/models":
                    self.respond(
                        {
                            "object": "list",
                            "data": [
                                {"id": "siq-synthetic-fixture", "object": "model", "owned_by": "fixture", "created": 0}
                            ],
                        }
                    )
                else:
                    self.send_error(404)

            def complete(self, message, finish, stream):
                payload = {"id": "siq-fixture", "created": 0, "model": "siq-synthetic-fixture"}
                if not stream:
                    self.respond(
                        {
                            **payload,
                            "object": "chat.completion",
                            "choices": [{"index": 0, "message": message, "finish_reason": finish}],
                            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
                        }
                    )
                    return
                if "tool_calls" in message:
                    message["tool_calls"][0]["index"] = 0
                chunks = [
                    {
                        **payload,
                        "object": "chat.completion.chunk",
                        "choices": [{"index": 0, "delta": message, "finish_reason": None}],
                    },
                    {
                        **payload,
                        "object": "chat.completion.chunk",
                        "choices": [{"index": 0, "delta": {}, "finish_reason": finish}],
                    },
                ]
                raw = "".join("data: " + json.dumps(chunk) + "\n\n" for chunk in chunks) + "data: [DONE]\n\n"
                self.send_response(200)
                self.send_header("Content-Type", "text/event-stream")
                self.send_header("Content-Length", str(len(raw.encode())))
                self.end_headers()
                self.wfile.write(raw.encode())

            def do_POST(self):
                try:
                    size = int(self.headers.get("Content-Length", "0"))
                    fixture.require(0 < size <= 2_000_000, "request budget")
                    body = json.loads(self.rfile.read(size))
                    if self.path == "/api/show":
                        self.send_error(404)
                        return
                    fixture.require(self.path == "/v1/chat/completions", "unexpected model route")
                    names = {item.get("function", {}).get("name") for item in body.get("tools", [])}
                    if "read_file" not in names:
                        fixture.require(len(auxiliary) < 16, "auxiliary request budget")
                        auxiliary.append({"stream": bool(body.get("stream"))})
                        self.complete(
                            {"role": "assistant", "content": "SIQ synthetic check"}, "stop", bool(body.get("stream"))
                        )
                        return
                    results = [item for item in body.get("messages", []) if item.get("role") == "tool"]
                    index = len(received)
                    fixture.require(
                        index <= len(calls) and len(results) == index,
                        f"unexpected model retry or lost history: step={index} results={len(results)}",
                    )
                    for call, result in zip(calls[:index], results, strict=True):
                        fixture.require(result["tool_call_id"] == call["id"], "tool call identity changed")
                        text = str(result.get("content", ""))
                        allowed = call["id"].startswith("allowed")
                        if allowed and expect_allowed_text:
                            if "fixture-visible-company-a" not in text:
                                category = "siq_block" if "siq-agent-security" in text else "host_result"
                                raise RuntimeError("allowed read missing: " + category)
                        else:
                            fixture.require("siq-agent-security" in text, "expected block missing")
                    message = {"role": "assistant", "content": "SIQ_RUNTIME_CHECK_COMPLETE"}
                    finish = "stop"
                    if index < len(calls):
                        call = calls[index]
                        message = {
                            "role": "assistant",
                            "content": None,
                            "tool_calls": [
                                {
                                    "id": call["id"],
                                    "type": "function",
                                    "function": {"name": call["tool"], "arguments": json.dumps(call["params"])},
                                }
                            ],
                        }
                        finish = "tool_calls"
                    received.append({"tool_result_count": len(results), "stream": bool(body.get("stream"))})
                    self.complete(message, finish, bool(body.get("stream")))
                except (RuntimeError, ValueError, TypeError, KeyError, OSError) as exc:
                    failures.append(type(exc).__name__ + ": " + str(exc)[:256])
                    self.send_error(500, "synthetic protocol failed")

        model = ThreadingHTTPServer(("127.0.0.1", 0), Model)
        thread = threading.Thread(target=model.serve_forever, daemon=True)
        thread.start()
        try:
            env = {
                **self.env,
                "HERMES_HOME": str(self.profile),
                "CUSTOM_BASE_URL": f"http://127.0.0.1:{model.server_port}/v1",
            }
            process = subprocess.run(
                [
                    str(self.args.hermes_cli),
                    "chat",
                    "--provider",
                    "custom",
                    "--model",
                    "siq-synthetic-fixture",
                    "--toolsets",
                    "file",
                    "--max-turns",
                    "6",
                    "--run-budget",
                    "45",
                    "--ignore-rules",
                    "--quiet",
                    "--oneshot",
                    "-q",
                    "Execute the SIQ synthetic runtime check.",
                ],
                cwd=self.workspace,
                env=env,
                capture_output=True,
                timeout=CLI_TIMEOUT,
                check=False,
            )
            fixture.require(
                process.returncode == 0,
                f"public CLI failed in {phase} phase: rc={process.returncode} failures={failures} received={received} out={process.stdout[-300:]!r} err={process.stderr[-300:]!r}",
            )
            fixture.require(not failures and len(received) == 4, f"{phase} conversation did not execute all probes")
            fixture.require(not forbidden.exists(), "forbidden write executed")
            return received
        finally:
            model.shutdown()
            model.server_close()
            thread.join(timeout=2)

    def signed_calls(self):
        calls = [
            {"id": "allowed-first", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/confirmation.txt")}},
            {"id": "write-denied", "tool": "write_file", "params": {"path": str(self.workspace / "company-a/must-not-exist.txt"), "content": "must not execute"}},
            {"id": "allowed-last", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/report.txt")}},
        ]
        self.run_cli(calls, "signed")
        records = self.receipts()
        fixture.require(len(records) == 5, "unexpected signed receipt count")
        intents = self.api("/v1/intents")["items"]
        fixture.require(len(intents) == 1, "manual or duplicate intent was created")
        self.native_intent = intents[0]
        self.signed_intent_id = self.native_intent["intent_id"]
        fixture.require(self.native_intent["authority"]["issuer"] == "local-runtime-identity", "wrong issuer")
        session = records[0]["session_id"]
        bindings = self.api("/v1/intent-bindings")["items"]
        fixture.require(len(bindings) == 1 and bindings[0]["session_id"] == session, "native session not bound")
        fixture.require(bindings[0]["grant_ref"] == self.issued["identity"]["grant_ref"], "wrong grant selection")
        for call in calls:
            self.assert_call(records, call["id"], "deny" if call["id"] == "write-denied" else "allow")
        return {"receipts": 5, "session_id": session, "checks": ["allowed_read", "write_denied_before_execution"]}

    def assert_call(self, records, call_id, outcome):
        own = [row for row in records if row.get("tool_call_id") == call_id]
        decisions = [row for row in own if row.get("record_type") == "decision"]
        fixture.require(len(decisions) == 1, "decision missing or duplicated")
        row = decisions[0]
        fixture.require(row["action"] == outcome and row["authority_status"] == "valid", "wrong decision")
        fixture.require(row["intent_id"] == self.native_intent["intent_id"], "wrong intent")
        fixture.require(row["agent_id"] == self.agent, "agent config overridden")
        fixture.require(row["matched_grant_id"] == self.issued["identity"]["grant_ref"]["grant_id"], "wrong grant")
        fixture.require(len(own) == (1 if outcome == "deny" else 2), "unexpected observations")
        if outcome == "deny":
            fixture.require(row["reason_code"] == "grant_scope_violation", "wrong rejection layer")

    def offline_block(self):
        pending_path = self.state / PENDING
        if pending_path.exists():
            pending_path.unlink()
        self.stop()
        calls = [
            {"id": "allowed-first", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/confirmation.txt")}},
            {"id": "write-denied", "tool": "write_file", "params": {"path": str(self.workspace / "company-a/must-not-exist.txt"), "content": "must not execute"}},
            {"id": "allowed-last", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/report.txt")}},
        ]
        self.run_cli(calls, "offline", offline=True)
        pending = [json.loads(line) for line in pending_path.read_text().splitlines()]
        fixture.require(
            len(pending) == 3 and all(row["outcome"] == "deny" for row in pending),
            "missing offline fail-closed evidence",
        )
        fixture.require(all(row["signed"] is False for row in pending), "offline denials must be unsigned")
        fixture.require(
            len({row["session_id"] for row in pending}) == 1 and pending[0]["session_id"] != self.signed_session,
            "offline run did not use a new native session",
        )
        self.offline_session = pending[0]["session_id"]
        fixture.require(
            all(row["enforcement_mode"] == "block" for row in pending), "offline denials must record block mode"
        )
        return {"pending_denials": len(pending), "session_id": pending[0]["session_id"]}

    def restart_and_recover(self):
        self.restart()
        calls = [
            {"id": "allowed-first", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/confirmation.txt")}},
            {"id": "write-denied", "tool": "write_file", "params": {"path": str(self.workspace / "company-a/must-not-exist.txt"), "content": "must not execute"}},
            {"id": "allowed-last", "tool": "read_file", "params": {"path": str(self.workspace / "company-a/report.txt")}},
        ]
        self.run_cli(calls, "recovered")
        records = self.receipts()
        fixture.require(len(records) == 13, f"unexpected receipt set after restart: {len(records)}")
        promoted = [row for row in records if row.get("record_type") is None]
        fixture.require(
            len(promoted) == 3
            and all(row.get("action") == "deny" and row.get("session_id") == self.offline_session for row in promoted),
            "offline denials were not promoted into the signed receipt chain",
        )
        recovered = [row for row in records if row.get("session_id") == records[-1]["session_id"]]
        fixture.require(len(recovered) == 5, "recovered receipt subset incomplete")
        intents = self.api("/v1/intents")["items"]
        fixture.require(len(intents) == 2, "recovered run did not create exactly one new intent")
        self.native_intent = next(i for i in intents if i["intent_id"] != self.signed_intent_id)
        fixture.require(
            all(row["session_id"] == records[-1]["session_id"] for row in recovered),
            "recovered receipts do not match the new native session",
        )
        fixture.require(len((self.state / PENDING).read_text().splitlines()) == 3, "restart produced new pending rows")
        for call in calls:
            self.assert_call(recovered, call["id"], "deny" if call["id"] == "write-denied" else "allow")
        return {
            "receipts_after_restart": len(records),
            "offline_denials_promoted": len(promoted),
            "session_id": records[-1]["session_id"],
        }

    def uninstall_and_restore(self):
        plan = self.api(
            "/v1/adapter/preview",
            {"platform": "hermes", "action": "uninstall", "instance_id": self.instance_id},
        )
        fixture.require(plan["action"] == "uninstall", "wrong uninstall plan")
        self.api(
            "/v1/adapter/uninstall",
            {
                "platform": "hermes",
                "instance_id": self.instance_id,
                "plan_id": plan["plan_id"],
                "plan_digest": plan["plan_digest"],
                "runtime_identity_id": plan["runtime_identity_id"],
                "actor_id": "automated-fixture-operator",
            },
        )
        identities = self.api("/v1/runtime-identities")["items"]
        status = next(
            i["status"] for i in identities if i["identity_id"] == self.issued["identity"]["identity_id"]
        )
        fixture.require(status == "revoked", "uninstall did not revoke the runtime identity")
        config = self.profile_config
        # With a real Hermes CLI on PATH the uninstall goes through the native
        # restore path (prepareHermesNative -> stage.restore), so the host
        # rewrites config.yaml in its own normalized form. The server guards
        # semantic restoration itself (nativeRegistration and nativeUnowned
        # DeepEqual in hermes_native.go), so here we assert semantics: the
        # product enablement is gone and every pre-install user setting
        # survives. Byte- and mode-exact restore without a native CLI is the
        # contract proven by internal/adapterinstall backup/restore tests.
        after = config.read_bytes()
        after_mode = stat.S_IMODE(os.stat(config).st_mode)
        fixture.require(
            b"siq-agent-security" not in after,
            "uninstall left product enablement in the profile config",
        )
        for keep in (
            b"model: " + "工作区".encode(),
            b"env: local",
            b"fixture_setting: named-profile-keep",
        ):
            fixture.require(keep in after, f"uninstall dropped user setting {keep!r}")
        fixture.require(
            after_mode & 0o022 == 0,
            "restored profile config is group/other writable",
        )
        removed = [profile / rel for profile in [self.profile] for rel in self.installed_files]
        fixture.require(
            all(not p.exists() for p in removed), "uninstall left product files behind"
        )
        # Top-level entries only: the Hermes CLI generates its own cache/logs/
        # skills trees during chat runs, which are host files we do not own.
        leftovers = sorted(p.name + ("/" if p.is_dir() else "") for p in self.profile.iterdir())
        fixture.require(
            self.work_config.read_bytes() == self.work_config_before
            and not (self.work_config.parent / "plugins/siq-agent-security").exists(),
            "default work profile changed during the journey",
        )
        verified = json.loads(self.command([str(self.binary), "verify"]))
        fixture.require(verified["verified"], "receipt chain invalid after uninstall")
        return {
            "config_restored_semantically": True,
            "config_mode_before": oct(self.config_mode_before),
            "config_mode_after": oct(after_mode),
            "native_cli_normalize_note": "restore runs through the real Hermes CLI, which rewrites config.yaml in normalized form; server-side guards assert semantic equality",
            "product_files_removed": len(removed),
            "profile_files_after_uninstall": leftovers,
            "identity_status": status,
        }

    def journey(self):
        discovery = self.discover()
        self.setup_authority()
        install = self.preview_and_install()
        signed = self.signed_calls()
        self.signed_session = signed["session_id"]
        offline = self.offline_block()
        recovered = self.restart_and_recover()
        restore = self.uninstall_and_restore()
        return {
            "schema_version": "personal-config-restore-journey-smoke/v1",
            "recorded_at": datetime.now(UTC).isoformat(),
            "passed": True,
            "binary_sha256": hashlib.sha256(self.binary.read_bytes()).hexdigest(),
            "harness_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            "native_cli_sha256": hashlib.sha256(self.args.hermes_cli.read_bytes()).hexdigest(),
            "native_entrypoint": "public hermes chat --oneshot against a named non-default profile",
            "discovery": discovery,
            "installation": install,
            "signed": signed,
            "offline_block": offline,
            "restart_recovery": recovered,
            "uninstall_restore": restore,
            "checks": [
                "non_ascii_named_profile_discovered_not_active",
                "default_work_profile_untouched_throughout",
                "preview_does_not_mutate_host",
                "managed_install_uses_issued_identity",
                "native_enable_preserves_user_settings",
                "allowed_read",
                "write_denied_before_execution",
                "offline_fail_closed_unsigned_denials",
                "daemon_restart_same_port_repairs_serving",
                "recovered_session_reuses_issued_identity",
                "uninstall_restores_config_semantically",
                "uninstall_revokes_runtime_identity",
                "receipt_chain_verified",
            ],
            "limitations": [
                "isolated named profile; browser approval and OS isolation are separate evidence",
                "synthetic model and operator; no real-user interaction",
                "product directories emptied of owned files may remain after uninstall (file-scoped ownership)",
                "config.yaml.siq-agent-security.orig recovery snapshot is retained after uninstall by design (immutable first-seen copy referenced by recovery guidance)",
            ],
        }


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--hermes-cli", type=Path, required=True)
    parser.add_argument("--binary", type=Path, help="prebuilt agentshield binary; built from source when omitted")
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    args.hermes_cli = args.hermes_cli.resolve()
    args.installer_managed_profile = True
    with tempfile.TemporaryDirectory(prefix="siq-config-restore-journey-") as temporary:
        root = Path(temporary)
        harness = Harness(root, args)
        home = root / "home"
        home.mkdir(mode=0o700, exist_ok=True)
        harness.env.update({"HOME": str(home), "USERPROFILE": str(home), "LOCALAPPDATA": str(home / "AppData/Local")})
        if args.binary:
            shutil.copy2(args.binary.resolve(), harness.binary)
        else:
            harness.build()
        try:
            harness.start()
            harness.create_named_profile()
            report = harness.journey()
        finally:
            harness.stop()
    args.out.parent.mkdir(parents=True, exist_ok=True)
    args.out.write_text(json.dumps(report, indent=2, ensure_ascii=False) + "\n")
    print(json.dumps(report, ensure_ascii=False))


if __name__ == "__main__":
    main()
