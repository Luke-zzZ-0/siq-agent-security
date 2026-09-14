"""Read installation bytes only; never import or execute WorkBuddy modules."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import struct

parser = argparse.ArgumentParser(
    description="Read WorkBuddy installation bytes only; never execute installed modules."
)
parser.add_argument(
    "--resources",
    type=Path,
    default=Path(os.environ.get("ProgramFiles", r"C:\Program Files")) / "WorkBuddy" / "resources",
    help="WorkBuddy resources directory (default: %%ProgramFiles%%/WorkBuddy/resources)",
)
parser.add_argument(
    "--out",
    type=Path,
    required=True,
    help="New output JSON path; parent directory must exist; existing files are never overwritten",
)
args = parser.parse_args()
resources = args.resources
archive = resources / "app.asar"
with archive.open("rb") as stream:
    words = struct.unpack("<4I", stream.read(16))
    assert words[0] == 4 and 0 < words[3] <= words[1] < 32 * 1024 * 1024
    index = json.loads(stream.read(words[3]))
payload_offset = 8 + words[1]
entries = {}


def walk(node, prefix=""):
    for name, entry in node.get("files", {}).items():
        path = prefix + name
        if "files" in entry:
            walk(entry, path + "/")
        else:
            entries[path] = entry


walk(index)
anchors = {
    "package.json": ['"version"'],
    "main/app-instance.js": ["process.env.CODEBUDDY_CONFIG_DIR = configDir", 'setPath("userData"', 'setPath("sessionData"'],
    "main/workbuddy-paths.js": ["function getWorkbuddyUserDataDir", "WORKBUDDY_USER_DATA_DIR", "function getWorkbuddySessionDataDir"],
    "main/workbuddy-product-config.js": ["function resolveWorkbuddyConfigDir", "function ensureWorkbuddyCustomUserDataDirEnv"],
    "main/index.js": ["acquireSingletonLock()", "function normalizeInheritedFallbackEnv"],
    "main/legacy-auth-session-migrator.js": ["function getLegacyWorkbuddyIdeVscdbPath", "APPDATA", "state.vscdb", "safeStorage.decryptString"],
    "main/launch-args.js": ["getLegacyAppDataDir", "legacy_history_migration", "legacy_mcp_oauth_migration", "WORKBUDDY_USER_DATA_DIR"],
    "main/node.js": ['args.push("--settings", JSON.stringify(settingsPayload))', "CLI_STATIC_MANAGED_ENV", "function buildCliProcessEnv", "readTextFile: false", "writeTextFile: false"],
    "main/tar.js": ["PLUGIN_METADATA_DIRS", "function mergeEnabledPlugins", "function readSessionSettingsSummary"],
    "main/sidecar-manager.js": ["cli/dist/codebuddy.js", '"--setting-sources"'],
    "main/src.js": ["--user-data-dir="],
    "main/e2b-filesystem.js": ["--workspace-folder"],
}


def summarize(name, raw, needles):
    content = raw.decode("utf-8")
    found = []
    for needle in needles:
        positions = []
        start = 0
        while True:
            offset = content.find(needle, start)
            if offset < 0:
                break
            positions.append({"character_offset": offset, "line_1based": content.count("\n", 0, offset) + 1})
            start = offset + len(needle)
        found.append({"literal": needle, "matches": positions})
    return {"path": name, "size": len(raw), "sha256": hashlib.sha256(raw).hexdigest(), "anchors": found}


modules = []
with archive.open("rb") as stream:
    for name, needles in anchors.items():
        entry = entries[name]
        assert not entry.get("unpacked")
        stream.seek(payload_offset + int(entry["offset"]))
        raw = stream.read(entry["size"])
        assert len(raw) == entry["size"]
        modules.append(summarize("app.asar/" + name, raw, needles))

unpacked = resources / "app.asar.unpacked"
file_anchors = {
    "cli/dist/codebuddy.js": ['"deny"===eu.permissionDecision', '"hooks/hooks.json"', "CODEBUDDY_DISABLE_EXTENDED_PLUGIN_HOOKS"],
    "cli/dist/web-ui/docs/cn/cli/hooks.md": ["### 简单方式", "其他退出代码", "#### PreToolUse 决策控制"],
    "cli/dist/web-ui/docs/cn/cli/plugins-reference.md": ["### 必需字段", "### 组件路径字段", ".workbuddy-plugin/plugin.json"],
    "resources/plugins/workbuddy-builtin/builtin-plugins/sheetagent/.codebuddy-plugin/plugin.json": ['"hooks"'],
    "resources/plugins/workbuddy-builtin/builtin-plugins/sheetagent/hooks/hooks.json": ["PreToolUse", "SubagentStop"],
}
for name, needles in file_anchors.items():
    modules.append(summarize("app.asar.unpacked/" + name, (unpacked / name).read_bytes(), needles))

desktop_flags = ["--user-data-dir", "--extensions-dir", "--profile", "--workspace"]
desktop_scan = []
main_count = 0
with archive.open("rb") as stream:
    for name, entry in entries.items():
        if not (name.startswith("main/") and name.endswith(".js") and not entry.get("unpacked")):
            continue
        main_count += 1
        stream.seek(payload_offset + int(entry["offset"]))
        raw = stream.read(entry["size"])
        content = raw.decode("utf-8")
        counts = {flag: content.count(flag) for flag in desktop_flags}
        if any(counts.values()):
            desktop_scan.append({"file": name, "sha256": hashlib.sha256(raw).hexdigest(), "literal_counts": counts})

product = json.loads((unpacked / "cli/product.json").read_text(encoding="utf-8"))
report = {
    "scope": "installation_source_review_only; no WorkBuddy execution or user configuration reads; not native acceptance",
    "archive_size": archive.stat().st_size,
    "archive_entries": len(entries),
    "installed_product_metadata_unverified": {key: product.get(key) for key in ("productName", "applicationName", "dataFolderName", "genieVersion", "commit", "date")},
    "metadata_file_sha256": {
        "app.asar.unpacked/cli/product.json": hashlib.sha256((unpacked / "cli/product.json").read_bytes()).hexdigest(),
        "app.asar.unpacked/cli/package.json": hashlib.sha256((unpacked / "cli/package.json").read_bytes()).hexdigest(),
    },
    "modules": modules,
    "flag_scan": {"packed_main_js_files_scanned": main_count, "matching_files": desktop_scan},
    "offset_unit": "Unicode code point index after UTF-8 decoding; lines count LF and are 1-based",
}
target = args.out
with target.open("x", encoding="utf-8", newline="\n") as stream:
    stream.write(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
print(json.dumps({"output": target.as_posix(), "modules": len(modules), "packed_main_js_files_scanned": main_count}))
