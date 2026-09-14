"""Strict, offline primitives for acceptance evidence; never execute a report."""

import hashlib
import json
from pathlib import Path, PurePosixPath


class Invalid(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise Invalid(message)


def read_json(path):
    def pairs(items):
        result = {}
        for key, value in items:
            require(key not in result, "duplicate JSON key")
            result[key] = value
        return result

    def constant(_):
        raise Invalid("non-finite JSON number")

    try:
        with Path(path).open("rb") as stream:
            raw = stream.read((8 << 20) + 1)
        require(len(raw) <= 8 << 20, "JSON size limit")
        value = json.loads(raw, object_pairs_hook=pairs, parse_constant=constant)
    except (OSError, UnicodeError, json.JSONDecodeError, RecursionError) as exc:
        raise Invalid("unreadable or malformed JSON") from exc
    require(isinstance(value, dict), "JSON object required")
    pending = [(value, 0)]
    while pending:
        node, depth = pending.pop()
        require(depth <= 64, "JSON nesting limit")
        if isinstance(node, dict):
            pending.extend((child, depth + 1) for child in node.values())
        elif isinstance(node, list):
            pending.extend((child, depth + 1) for child in node)
    return value


def sha256(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def safe_ref(repo, ref):
    require(isinstance(ref, str) and ref and "\\" not in ref and ":" not in ref,
            "invalid evidence ref")
    parts = PurePosixPath(ref).parts
    require(not PurePosixPath(ref).is_absolute() and ".." not in parts
            and parts[:2] == ("docs", "evidence")
            and PurePosixPath(ref).as_posix() == ref, "unsafe evidence ref")
    root = Path(repo).resolve()
    path = root
    try:
        for part in parts:
            path = path / part
            require(not path.is_symlink(), "symlink evidence ref")
        resolved = path.resolve(strict=True)
        require(resolved.is_relative_to(root / "docs" / "evidence"),
                "evidence outside docs/evidence")
        require(resolved.is_file() and resolved.stat().st_size <= 8 << 20,
                "missing or oversized evidence file")
    except OSError as exc:
        raise Invalid("unreadable evidence ref") from exc
    return resolved


def passed_checks(report):
    # A top-level green flag cannot override a failed nested assertion.
    require(report.get("passed") is True or report.get("status") == "passed",
            "report not passed")
    if "passed" in report:
        require(report["passed"] is True, "contradictory report verdict")
    if "status" in report:
        require(report["status"] == "passed", "contradictory report status")
    checks = report.get("checks")
    require(isinstance(checks, (list, dict)) and checks, "nonempty checks required")
    names = list(checks)
    require(all(isinstance(name, str) and name for name in names), "check names")
    require(len(set(names)) == len(names), "duplicate check names")
    if isinstance(checks, dict):
        require(all(value is True for value in checks.values()), "failed report check")
    journey = report.get("journey_checks")
    if journey is not None:
        require(isinstance(journey, dict), "journey_checks type")
        require(all(isinstance(journey.get(name), dict)
                    and journey[name].get("passed") is True for name in names),
                "missing or failed journey check")
        require(all(not isinstance(value, dict) or "passed" not in value
                    or (value["passed"] is True and name in names)
                    for name, value in journey.items()), "unlisted or failed journey check")
    return set(names)


def report_binary(report):
    runtime = report.get("runtime", {})
    require(isinstance(runtime, dict), "runtime type")
    values = [report.get("binary_sha256"), report.get("candidate_sha256"),
              runtime.get("agentshield_binary_sha256")]
    values = [value for value in values if value is not None]
    require(values and all(value == values[0] for value in values),
            "missing or contradictory report binary")
    return values[0]


EXPECTED_CHECKS = frozenset({
    "C1_daemon_start_and_pair", "C2_preview_no_mutation_plan_v3",
    "C3_managed_install_lands", "C4_real_runtime_loads_plugin",
    "C5_allow_path_real_agent_turn", "C6a_real_turn_held_and_blocked",
    "C6b_console_approve_then_consume", "C7_fail_closed_when_daemon_down",
    "C8_receipts_chain_verified_runtime_identity", "C9_managed_uninstall_restores",
    "C10_hygiene",
})


def checks_complete(checks, journey):
    """Completion requires every named assertion, never all([]) or a prefix."""
    return (isinstance(checks, list) and len(checks) == len(EXPECTED_CHECKS)
            and all(isinstance(name, str) for name in checks)
            and set(checks) == EXPECTED_CHECKS and isinstance(journey, dict)
            and all(isinstance(journey.get(name), dict)
                    and journey[name].get("passed") is True for name in EXPECTED_CHECKS)
            and all(not isinstance(value, dict) or "passed" not in value
                    or (name in EXPECTED_CHECKS and value["passed"] is True)
                    for name, value in journey.items()))
