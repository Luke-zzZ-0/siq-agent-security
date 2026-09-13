#!/usr/bin/env python3
"""Validate embedded task activity UI with isolated daemon and explicit response fixtures."""
import argparse
import importlib.util
import json
import re
import shutil
import tempfile
from pathlib import Path
from urllib.parse import parse_qs, urlparse

from playwright.sync_api import expect, sync_playwright

REPO = Path(__file__).resolve().parents[2]
loader = importlib.util.spec_from_file_location("daemon_fixture", REPO / "scripts/validate-intent-v2-hermes.py")
fixture = importlib.util.module_from_spec(loader)
loader.loader.exec_module(fixture)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--out-dir", required=True, type=Path)
    args = parser.parse_args()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="siq-activities-browser-") as temp:
        root = Path(temp)
        args.installer_managed_profile = True
        args.hermes_cli = root / "unavailable-hermes-cli"
        h = fixture.Harness(root, args)
        shutil.copy2(args.binary.resolve(), h.binary)
        try:
            h.start()
            pairing = h.command([str(h.binary), "pair", "--port", h.endpoint.rsplit(":", 1)[1]])
            code = re.search(r"\b[0-9a-f]{4}(?:-[0-9a-f]{4}){3}\b", pairing).group()
            with sync_playwright() as pw:
                browser = pw.chromium.launch(headless=True)
                page = browser.new_page(viewport={"width": 1440, "height": 1000}, locale="zh-CN")
                errors = []
                page.on("pageerror", lambda error: errors.append(str(error)))
                page.goto(h.endpoint + "/activities")
                page.get_by_label("配对码", exact=True).fill(code)
                page.get_by_role("button", name="建立管理会话", exact=True).click()
                expect(page.get_by_text("尚无已归属任务，可切换查看未归属活动。", exact=True)).to_be_visible()
                sample = json.loads((REPO / "apps/agentshield/testdata/contracts/local-task-activities.json").read_text())
                mode = "populated"

                def activity(route):
                    if mode == "changed":
                        route.fulfill(status=409, json={"error": "task_activity_snapshot_changed"})
                        return
                    if mode == "unavailable":
                        route.fulfill(status=500, json={"error": "task_activity_snapshot_unavailable"})
                        return
                    query = parse_qs(urlparse(route.request.url).query)
                    view = query.get("view", ["tasks"])[0]
                    data = {**sample, "view": view, "total": 1, "next_offset": None}
                    item = {**sample["items"][0]}
                    if view == "unassigned":
                        item.update(attribution="unknown", binding=None, receipt_count=1)
                    data["items"] = [item]
                    route.fulfill(json=data)

                search_reads = []
                def activity_search(route):
                    search_reads.append(route.request.url)
                    if mode == "changed":
                        route.fulfill(status=409, json={"error": "task_activity_snapshot_changed"})
                        return
                    if mode == "unavailable":
                        route.fulfill(status=500, json={"error": "task_activity_snapshot_unavailable"})
                        return
                    query = parse_qs(urlparse(route.request.url).query, keep_blank_values=True)
                    view = query.get("view", ["tasks"])[0]
                    filters = {name: query.get(name, [""])[0] for name in ["platform", "agent_id", "session_id", "task_id", "q"]}
                    if mode == "search-empty":
                        route.fulfill(json={**sample, "schema_version": "local-task-activity-search/v1", "view": view,
                                            "total": 0, "next_offset": None, "items": [], "filters": filters})
                        return
                    data = {**sample, "schema_version": "local-task-activity-search/v1", "view": view,
                            "total": 1, "next_offset": None, "filters": filters}
                    item = {**sample["items"][0]}
                    if view == "unassigned":
                        item.update(attribution="unknown", binding=None, receipt_count=1)
                    data["items"] = [item]
                    route.fulfill(json=data)

                detail_sample = json.loads((REPO / "apps/agentshield/testdata/contracts/local-task-activity-detail.json").read_text())

                def detail_response(route):
                    if mode == "missing":
                        route.fulfill(status=404, json={"error": "task_activity_not_found"})
                        return
                    item = sample["items"][0]
                    data = {**detail_sample, "snapshot": sample["snapshot"], "activity": item,
                            "total": 1, "next_offset": None}
                    data["receipts"] = [{**detail_sample["receipts"][0], "agent_id": item["binding"]["agent_id"]}]
                    route.fulfill(json=data)

                def completion_response(route):
                    if mode == "effect-unavailable":
                        route.fulfill(status=500, json={"error": "completion_evidence_unavailable"})
                        return
                    item = sample["items"][0]
                    route.fulfill(json={
                        "schema_version": "local-task-activity-completion/v1", "snapshot": sample["snapshot"],
                        "activity": item, "evaluated_at": "2026-09-12T00:00:00Z", "reason_code": "evaluated",
                        "result": {"schema_version": "completion-status/v1", "task_id": item["binding"]["task_id"],
                                   "status": "verified", "reason_code": "effects_verified", "incident_ids": [],
                                   "requirements": [{"requirement_id": "write-report", "status": "verified",
                                                     "reason_code": "effect_verified", "evidence_ids": ["evidence-fixture-1"]}]},
                    })

                page.route("**/v1/task-activities/*?*", detail_response)
                page.route("**/v1/task-activities/*/completion?*", completion_response)
                source_reads = []
                def sources_response(route):
                    source_reads.append(route.request.url)
                    if mode == "sources-unavailable":
                        route.fulfill(status=500, json={"error": "unavailable"})
                        return
                    data = json.loads((REPO / "apps/agentshield/testdata/contracts/local-task-activity-sources.json").read_text())
                    item = sample["items"][0]
                    data.update(activity_id=item["activity_id"], snapshot=sample["snapshot"], total=1, next_offset=None)
                    data["items"][0]["receipt_hash"] = detail_sample["receipts"][0]["hash"]
                    data["items"][0]["source"]["grant_id"] = "grant-a"
                    route.fulfill(json=data)
                page.route("**/v1/task-activities/*/sources?*", sources_response)
                export_reads = []
                def export_response(route):
                    export_reads.append(route.request.url)
                    if mode == "export-changed":
                        route.fulfill(status=409, json={"error": "task_activity_snapshot_changed"})
                        return
                    data = json.loads((REPO / "apps/agentshield/testdata/contracts/local-task-activity-export.json").read_text())
                    item = sample["items"][0]
                    data.update(activity_id=item["activity_id"], snapshot=sample["snapshot"])
                    data["receipts"] = [data["receipts"][0]]
                    route.fulfill(json=data)
                page.route("**/v1/task-activities/*/export?*", export_response)
                trace_reads = []
                def trace_response(route):
                    trace_reads.append(route.request.url)
                    if mode == "trace-changed":
                        route.fulfill(status=409, json={"error": "task_activity_trace_changed"})
                        return
                    data = json.loads((REPO / "apps/agentshield/testdata/contracts/local-task-trace-export.json").read_text())
                    item = sample["items"][0]
                    data.update(activity_id=item["activity_id"], snapshot=sample["snapshot"])
                    route.fulfill(json=data)
                page.route("**/v1/task-activities/*/trace-export?*", trace_response)
                evidence_reads = []
                def evidence_response(route):
                    evidence_reads.append(route.request.url)
                    if mode == "evidence-unavailable":
                        route.fulfill(status=500, json={"error": "effect_evidence_state_unavailable"})
                        return
                    evidence = json.loads((REPO / "apps/agentshield/testdata/contracts/effect-evidence.sample.json").read_text())
                    evidence["effect_evidence_id"] = "evidence-fixture-1"
                    route.fulfill(json={"schema_version": "effect-evidence-record/v1", "task_id": "t",
                                        "evidence": evidence, "finding_code": "", "request_digest": "a" * 64,
                                        "signing_schema": "local_canonical/v1", "signature": "b" * 128})
                page.route("**/v1/effect-evidence/*", evidence_response)

                page.route("**/v1/task-activities?*", activity)
                page.route("**/v1/task-activities/search?*", activity_search)
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_role("cell", name="agent-1", exact=True)).to_be_visible()
                expect(page.get_by_text("尚无法确认完整历史是否存在缺失。", exact=False)).to_be_visible()
                assert not search_reads
                page.get_by_label("关键词", exact=True).fill("agent-1")
                assert not search_reads
                page.get_by_role("button", name="应用筛选", exact=True).click()
                expect(page.get_by_text("已按标识字段筛选，共 1 项。", exact=True)).to_be_visible()
                assert "q=agent-1" in page.url and len(search_reads) == 1
                page.screenshot(path=str(args.out_dir / "desktop.png"), full_page=True, animations="disabled")
                page.get_by_role("link", name="查看活动记录", exact=True).click()
                assert "q=agent-1" in page.url
                expect(page.get_by_role("heading", name="活动详情", exact=True)).to_be_visible()
                expect(page.get_by_role("cell", name="read_file", exact=True)).to_be_visible()
                expect(page.get_by_role("cell", name="grant-a", exact=True)).to_be_visible()
                assert not source_reads
                page.get_by_role("button", name="查看历史 Skill 来源", exact=True).click()
                source_panel = page.get_by_role("region", name="历史 Skill 来源", exact=True)
                expect(source_panel).to_contain_text("history-skill · 声明版本：1.2.3")
                page.screenshot(path=str(args.out_dir / "sources-desktop.png"), full_page=True, animations="disabled")
                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function("document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                source_panel.scroll_into_view_if_needed()
                page.screenshot(path=str(args.out_dir / "sources-mobile.png"), full_page=True, animations="disabled")
                page.set_viewport_size({"width": 1440, "height": 1000})
                page.get_by_role("button", name="关闭历史 Skill 来源", exact=True).click()
                mode = "sources-unavailable"
                page.get_by_role("button", name="查看历史 Skill 来源", exact=True).click()
                expect(source_panel.get_by_role("alert")).to_contain_text("历史来源当前不可读取")
                expect(source_panel.get_by_text("history-skill", exact=False)).to_have_count(0)
                page.get_by_role("button", name="关闭历史 Skill 来源", exact=True).click()
                mode = "populated"
                assert not export_reads
                assert not trace_reads
                export_button = page.get_by_role("button", name="下载脱敏回执摘要", exact=True)
                with page.expect_download() as download_info:
                    export_button.click()
                download = download_info.value
                assert download.suggested_filename == "siq-activity-" + sample["items"][0]["activity_id"] + ".json"
                download.save_as(str(args.out_dir / "activity-export.json"))
                exported = json.loads((args.out_dir / "activity-export.json").read_text())
                assert len(exported["receipts"]) == 1 and exported["attestation_scope"] == "share_projection_only"
                mode = "export-changed"
                export_button.click()
                expect(page.get_by_role("region", name="活动摘要导出").get_by_role("alert")).to_contain_text("请刷新详情后重新下载")
                mode = "populated"
                trace_button = page.get_by_role("button", name="下载完整脱敏追溯包", exact=True)
                expect(trace_button).to_have_count(1)
                expect(page.get_by_role("region", name="完整追溯包导出", exact=True)).to_have_count(1)
                with page.expect_download() as download_info:
                    trace_button.click()
                download = download_info.value
                assert download.suggested_filename == "siq-trace-" + sample["items"][0]["activity_id"] + ".json"
                download.save_as(str(args.out_dir / "task-trace-export.json"))
                trace = json.loads((args.out_dir / "task-trace-export.json").read_text())
                assert trace["attestation_scope"] == "redacted_trace_projection_only" and trace["incomplete"] is True
                trace_panel = page.get_by_role("region", name="完整追溯包导出", exact=True)
                expect(trace_panel.get_by_role("status")).to_contain_text("材料不完整")
                trace_panel.scroll_into_view_if_needed()
                page.screenshot(path=str(args.out_dir / "trace-desktop.png"), animations="disabled")
                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function("document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                trace_panel.scroll_into_view_if_needed()
                page.screenshot(path=str(args.out_dir / "trace-mobile.png"), animations="disabled")
                page.set_viewport_size({"width": 1440, "height": 1000})
                mode = "trace-changed"
                trace_button.click()
                expect(page.get_by_role("region", name="完整追溯包导出").get_by_role("alert")).to_contain_text("请刷新详情后重新下载")
                mode = "populated"
                panel = page.get_by_role("region", name="实际效果核验", exact=True)
                expect(panel.get_by_text("效果已核验", exact=True)).to_be_visible()
                expect(panel).to_contain_text("evidence-fixture-1")
                assert not evidence_reads
                evidence_button = page.get_by_role("button", name="查看证据 evidence-fixture-1", exact=True)
                evidence_button.click()
                evidence_panel = page.get_by_role("region", name="效果证据详情", exact=True)
                expect(evidence_panel).to_contain_text("observer-1")
                expect(evidence_panel).to_contain_text("宿主独立观测")
                page.screenshot(path=str(args.out_dir / "evidence-desktop.png"), full_page=True, animations="disabled")
                page.get_by_role("button", name="关闭证据详情", exact=True).click()
                expect(evidence_button).to_be_focused()
                mode = "evidence-unavailable"
                evidence_button.click()
                expect(evidence_panel.get_by_role("alert")).to_contain_text("这份证据当前不可读取")
                page.get_by_role("button", name="关闭证据详情", exact=True).click()
                mode = "populated"

                mode = "effect-unavailable"
                page.get_by_role("button", name="刷新详情", exact=True).click()
                expect(panel.get_by_role("alert")).to_contain_text("效果证据当前不可用")
                expect(panel.get_by_text("效果已核验", exact=True)).to_have_count(0)
                expect(page.get_by_role("cell", name="read_file", exact=True)).to_be_visible()
                mode = "populated"
                page.get_by_role("button", name="刷新详情", exact=True).click()
                expect(panel.get_by_text("效果已核验", exact=True)).to_be_visible()

                page.screenshot(path=str(args.out_dir / "detail-desktop.png"), full_page=True, animations="disabled")
                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function("document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                page.screenshot(path=str(args.out_dir / "detail-mobile.png"), full_page=True, animations="disabled")
                page.set_viewport_size({"width": 1440, "height": 1000})

                mode = "missing"
                page.get_by_role("button", name="刷新详情", exact=True).click()
                expect(page.get_by_text("未找到该活动，请返回活动列表重新选择。", exact=True).last).to_be_visible()
                expect(page.get_by_role("cell", name="read_file", exact=True)).to_have_count(0)
                mode = "populated"
                page.get_by_role("link", name="返回活动列表", exact=True).click()
                expect(page.get_by_role("cell", name="agent-1", exact=True)).to_be_visible()
                expect(page.get_by_label("关键词", exact=True)).to_have_value("agent-1")
                assert "q=agent-1" in page.url
                mode = "search-empty"
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_text("没有匹配当前筛选条件的活动。", exact=True)).to_be_visible()
                expect(page.get_by_role("cell", name="agent-1", exact=True)).to_have_count(0)
                mode = "populated"
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_role("cell", name="agent-1", exact=True)).to_be_visible()

                page.get_by_role("button", name="未归属活动", exact=True).click()
                expect(page.get_by_role("cell", name="缺少可信绑定", exact=True)).to_be_visible()
                assert "view=unassigned" in page.url
                page.reload()
                expect(page.get_by_role("cell", name="缺少可信绑定", exact=True)).to_be_visible()
                mode = "changed"
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_text("活动记录已更新，请点击“刷新活动”重新加载。", exact=True).last).to_be_visible()
                expect(page.get_by_role("cell", name="缺少可信绑定", exact=True)).to_have_count(0)
                mode = "unavailable"
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_text("活动记录当前不可用。", exact=True)).to_be_visible()
                mode = "populated"
                page.get_by_role("button", name="刷新活动", exact=True).click()
                expect(page.get_by_role("cell", name="缺少可信绑定", exact=True)).to_be_visible()
                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function("document.querySelector('.sidebar').getBoundingClientRect().right <= 0")
                page.screenshot(path=str(args.out_dir / "mobile.png"), full_page=True, animations="disabled")
                assert not errors, errors
                browser.close()
                (args.out_dir / "result.json").write_text(json.dumps({
                    "real_daemon_pairing_and_empty_list": True,
                    "fixture_populated_unknown_refresh_and_errors": True,
                    "fixture_detail_receipts_grant_reference_missing_and_return": True,
                    "fixture_effect_verified_evidence_and_failure_removes_success": True,
                    "fixture_evidence_on_demand_metadata_error_and_focus_return": True,
                    "fixture_activity_download_and_snapshot_conflict": True,
                    "fixture_complete_trace_download_incomplete_state_and_conflict": True,
                    "fixture_historical_sources_on_demand_and_failure": True,
                    "fixture_search_submit_url_restore_and_errors": True,
                    "native_platform_acceptance": False,
                    "page_errors": errors,
                }, indent=2) + "\n")
        finally:
            h.stop()


if __name__ == "__main__":
    main()
