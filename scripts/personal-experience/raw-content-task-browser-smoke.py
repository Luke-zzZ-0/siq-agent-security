"""Exercise task-scoped raw grants and explicit record actions in the embedded UI."""

import argparse
import hashlib
import importlib.util
import json
import re
import shutil
import tempfile
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

REPO = Path(__file__).resolve().parents[2]
loader = importlib.util.spec_from_file_location(
    "daemon_fixture", REPO / "scripts/validate-intent-v2-hermes.py"
)
fixture = importlib.util.module_from_spec(loader)
loader.loader.exec_module(fixture)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", required=True, type=Path)
    parser.add_argument("--out-dir", required=True, type=Path)
    args = parser.parse_args()
    args.binary = args.binary.resolve()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    args.installer_managed_profile = True

    with tempfile.TemporaryDirectory(prefix="siq-raw-task-browser-") as temp:
        root = Path(temp)
        args.hermes_cli = root / "unavailable-hermes-cli"
        harness = fixture.Harness(root, args)
        shutil.copy2(args.binary, harness.binary)
        checks = {}
        try:
            harness.start()
            harness.api(
                "/v1/raw-task-content/activation",
                {
                    "schema_version": "local-raw-task-content-activate/v1",
                    "actor_id": "fixture-setup",
                    "retention_seconds": 604800,
                    "budget_bytes": 64 << 20,
                },
                expected=201,
            )
            pairing = harness.command(
                [str(harness.binary), "pair", "--port", harness.endpoint.rsplit(":", 1)[1]]
            )
            code = re.search(r"\b[0-9a-f]{4}(?:-[0-9a-f]{4}){3}\b", pairing).group()
            activity_page = json.loads(
                (REPO / "apps/agentshield/testdata/contracts/local-task-activities.json").read_text()
            )
            activity = {**activity_page["items"][0]}
            activity["binding"] = {**activity["binding"], "task_id": "fixture-runtime-task"}
            detail_fixture = json.loads(
                (REPO / "apps/agentshield/testdata/contracts/local-task-activity-detail.json").read_text()
            )
            receipt = {
                **detail_fixture["receipts"][0],
                "agent_id": activity["binding"]["agent_id"],
                "platform": activity["binding"]["platform"],
                "session_id": activity["binding"]["session_id"],
                "seq": activity["first_seq"],
            }
            detail = {
                **detail_fixture,
                "snapshot": activity_page["snapshot"],
                "activity": activity,
                "total": 1,
                "next_offset": None,
                "receipts": [receipt],
            }
            record = json.loads(
                (REPO / "apps/agentshield/testdata/contracts/local-raw-task-content-records.json").read_text()
            )["items"][0]
            task_ref = "sha256:" + hashlib.sha256(b"fixture-runtime-task").hexdigest()
            record = {**record, "task_ref": task_ref}
            show_record = False
            record_deleted = False
            invalid_records = False
            record_reads = []

            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(headless=True)
                context = browser.new_context(
                    viewport={"width": 1440, "height": 1100}, locale="zh-CN"
                )
                page = context.new_page()
                page_errors = []
                page.on("pageerror", lambda error: page_errors.append(str(error)))

                page.route(
                    f"**/v1/task-activities/{activity['activity_id']}?*",
                    lambda route: route.fulfill(json=detail),
                )

                def completion_response(route):
                    route.fulfill(
                        json={
                            "schema_version": "local-task-activity-completion/v1",
                            "snapshot": activity_page["snapshot"],
                            "activity": activity,
                            "evaluated_at": "2026-09-13T00:00:00Z",
                            "reason_code": "evaluated",
                            "result": {
                                "schema_version": "completion-status/v1",
                                "task_id": "fixture-runtime-task",
                                "status": "unknown",
                                "reason_code": "not_required",
                                "incident_ids": [],
                                "requirements": [],
                            },
                        }
                    )

                page.route("**/v1/task-activities/*/completion?*", completion_response)

                def records_response(route):
                    items = [record] if show_record and not record_deleted else []
                    if invalid_records:
                        items = [{**record, "task_ref": "sha256:" + "0" * 64}]
                    route.fulfill(
                        json={
                            "schema_version": "local-raw-task-content-records/v1",
                            "items": items,
                        }
                    )

                def read_response(route):
                    record_reads.append(route.request.url)
                    route.fulfill(
                        json={
                            "schema_version": "local-raw-task-content-record-content/v1",
                            "contains_plaintext": True,
                            "record": record,
                            "fields": [{"path": "/prompt", "value": "synthetic fixture prompt"}],
                        }
                    )

                def delete_response(route):
                    nonlocal record_deleted
                    record_deleted = True
                    route.fulfill(
                        json={
                            "schema_version": "local-raw-task-content-record-deleted/v1",
                            "record_id": record["record_id"],
                            "deleted": True,
                        }
                    )

                page.route("**/v1/raw-task-content/records/search", records_response)
                page.route(f"**/v1/raw-task-content/records/{record['record_id']}/read", read_response)
                page.route(f"**/v1/raw-task-content/records/{record['record_id']}/delete", delete_response)

                page.goto(harness.endpoint + f"/activities/{activity['activity_id']}?view=tasks")
                page.get_by_label("配对码", exact=True).fill(code)
                page.get_by_role("button", name="建立管理会话", exact=True).click()
                panel = page.get_by_role("region", name="任务原文管理", exact=True)
                expect(panel.get_by_text("此任务尚无原文采集授权。", exact=True)).to_be_visible()
                expect(panel.get_by_text("授权本身不会创建记录。", exact=False)).to_be_visible()
                checks["ready_task_starts_without_capture_grant"] = True

                panel.get_by_label("输入", exact=True).check()
                panel.get_by_label("结果", exact=True).check()
                panel.get_by_label("授权有效期", exact=True).select_option("3600")
                panel.get_by_label("记录保留期", exact=True).select_option("86400")
                panel.get_by_label("单条原文上限", exact=True).select_option("65536")
                panel.get_by_label("我确认仅为当前任务授权", exact=False).check()
                panel.get_by_role("button", name="创建任务原文授权", exact=True).click()
                expect(panel.get_by_text("采集授权有效", exact=True)).to_be_visible()
                grants = harness.api("/v1/raw-task-content/grants")["items"]
                fixture.require(
                    len(grants) == 1
                    and grants[0]["grant"]["task_ref"] == task_ref
                    and grants[0]["grant"]["kinds"] == ["input", "output"]
                    and grants[0]["grant"]["retention_seconds"] == 86400,
                    "task UI created a grant outside the selected scope",
                )
                checks["explicit_task_grant_uses_real_signed_api"] = True

                panel.get_by_role("button", name="撤销这份采集授权", exact=True).click()
                revoke_region = panel.get_by_role("region", name="撤销原文授权", exact=True)
                expect(revoke_region.get_by_role("button", name="确认撤销", exact=True)).to_be_disabled()
                revoke_region.get_by_label("确认撤销此任务", exact=False).check()
                revoke_region.get_by_role("button", name="确认撤销", exact=True).click()
                expect(panel.get_by_text("授权已撤销", exact=True)).to_be_visible()
                fixture.require(
                    harness.api("/v1/raw-task-content/grants")["items"][0]["status"] == "revoked",
                    "task UI did not create terminal revocation",
                )
                checks["explicit_revocation_uses_current_grant_signature"] = True

                show_record = True
                panel.get_by_role("button", name="刷新原文状态", exact=True).click()
                expect(panel.get_by_text(record["record_id"], exact=True)).to_be_visible()
                panel.get_by_role("button", name="二次确认后查看", exact=True).click()
                read_region = panel.get_by_role("region", name="查看任务原文", exact=True)
                fixture.require(not record_reads, "opening confirmation fetched plaintext")
                expect(read_region.get_by_role("button", name="读取一次", exact=True)).to_be_disabled()
                read_region.get_by_label("我确认现在把这条记录", exact=False).check()
                read_region.get_by_role("button", name="读取一次", exact=True).click()
                expect(read_region.get_by_text("synthetic fixture prompt", exact=True)).to_be_visible()
                fixture.require(len(record_reads) == 1, "confirmed read did not issue exactly one request")
                checks["plaintext_requires_second_action_and_is_scoped_to_record"] = True
                read_region.get_by_role("button", name="关闭并清除", exact=True).click()
                expect(page.get_by_text("synthetic fixture prompt", exact=True)).to_have_count(0)
                checks["closing_plaintext_removes_it_from_dom"] = True

                panel.screenshot(
                    path=str(args.out_dir / "raw-task-panel-desktop.png"), animations="disabled"
                )
                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function(
                    "document.querySelector('.sidebar').getBoundingClientRect().right <= 0"
                )
                panel.scroll_into_view_if_needed()
                fixture.require(
                    page.evaluate("document.documentElement.scrollWidth <= innerWidth"),
                    "task raw-content panel causes mobile horizontal overflow",
                )
                panel.screenshot(
                    path=str(args.out_dir / "raw-task-panel-mobile.png"), animations="disabled"
                )
                panel.get_by_text("此任务的原文记录", exact=True).scroll_into_view_if_needed()
                page.screenshot(
                    path=str(args.out_dir / "raw-task-records-mobile.png"),
                    animations="disabled",
                )
                checks["mobile_task_panel_has_no_horizontal_overflow"] = True
                page.set_viewport_size({"width": 1440, "height": 1100})

                panel.get_by_role("button", name="删除这条密文", exact=True).click()
                delete_region = panel.get_by_role("region", name="删除任务原文", exact=True)
                expect(delete_region.get_by_role("button", name="确认删除密文", exact=True)).to_be_disabled()
                delete_region.get_by_label("确认删除完整 ID", exact=False).check()
                delete_region.get_by_role("button", name="确认删除密文", exact=True).click()
                expect(panel.get_by_text("授权本身不会创建记录。", exact=False)).to_be_visible()
                checks["record_delete_requires_full_id_confirmation"] = True

                invalid_records = True
                record_deleted = False
                panel.get_by_role("button", name="刷新原文状态", exact=True).click()
                expect(panel.get_by_role("alert")).to_contain_text("当前任务或选择不匹配")
                expect(panel.get_by_text("授权已撤销", exact=True)).to_have_count(0)
                expect(panel.get_by_text(record["record_id"], exact=True)).to_have_count(0)
                checks["cross_task_record_response_removes_old_trusted_state"] = True

                storage = page.evaluate(
                    "JSON.stringify({local: {...localStorage}, session: {...sessionStorage}})"
                )
                fixture.require(
                    "synthetic fixture prompt" not in storage
                    and record["record_id"] not in storage
                    and grants[0]["grant"]["signature"] not in storage,
                    "task raw content or authority leaked into browser storage",
                )
                checks["plaintext_and_authority_are_not_browser_persisted"] = True
                fixture.require(not page_errors, "browser page error: " + "; ".join(page_errors))
                browser.close()

            (args.out_dir / "result.json").write_text(
                json.dumps({**checks, "page_errors": page_errors}, ensure_ascii=False, indent=2)
                + "\n"
            )
        finally:
            harness.stop()


if __name__ == "__main__":
    main()
