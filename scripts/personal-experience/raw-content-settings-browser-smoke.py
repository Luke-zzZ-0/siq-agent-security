"""Exercise raw-content status, activation and expired purge in the embedded local UI."""

import argparse
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

    with tempfile.TemporaryDirectory(prefix="siq-raw-content-browser-") as temp:
        root = Path(temp)
        args.hermes_cli = root / "unavailable-hermes-cli"
        harness = fixture.Harness(root, args)
        shutil.copy2(args.binary, harness.binary)
        checks = {}
        try:
            harness.start()
            pairing = harness.command(
                [str(harness.binary), "pair", "--port", harness.endpoint.rsplit(":", 1)[1]]
            )
            code = re.search(r"\b[0-9a-f]{4}(?:-[0-9a-f]{4}){3}\b", pairing).group()
            with sync_playwright() as playwright:
                browser = playwright.chromium.launch(headless=True)
                context = browser.new_context(
                    viewport={"width": 1440, "height": 1000}, locale="zh-CN"
                )
                page = context.new_page()
                page_errors = []
                page.on("pageerror", lambda error: page_errors.append(str(error)))
                page.goto(harness.endpoint + "/settings")
                page.get_by_label("配对码", exact=True).fill(code)
                page.get_by_role("button", name="建立管理会话", exact=True).click()

                panel = page.locator(".raw-content-privacy")
                expect(panel.get_by_role("heading", name="按需原文与本机存储", exact=True)).to_be_visible()
                expect(panel.get_by_text("原文记录关闭。", exact=True)).to_be_visible()
                expect(page.get_by_text("原文记录关闭", exact=True)).to_be_visible()
                fixture.require(
                    harness.api("/v1/raw-task-content/status")["status"] == "disabled",
                    "fresh isolated daemon did not start with raw capture disabled",
                )
                checks["fresh_install_is_disabled_and_visible"] = True

                page.get_by_label("批准 / hold 签核使用的 actor_id", exact=False).fill(
                    "browser-privacy-operator"
                )
                panel.get_by_label("原文保留期", exact=True).select_option("604800")
                panel.get_by_label("最大磁盘占用", exact=True).select_option(str(256 << 20))
                panel.get_by_label("我理解原文是独立的可删除辅助内容", exact=False).check()
                panel.get_by_role("button", name="启用按需原文仓", exact=True).click()
                expect(panel.get_by_text("原文仓已启用，默认采集仍为关闭。", exact=True)).to_be_visible()
                expect(panel.get_by_text("7 天", exact=True)).to_be_visible()
                expect(panel.get_by_text("256 MiB", exact=True)).to_be_visible()
                expect(page.get_by_text("原文仓已启用 · 按任务授权", exact=True)).to_be_visible()
                status = harness.api("/v1/raw-task-content/status")
                fixture.require(
                    status["status"] == "ready"
                    and status["default_capture"] is False
                    and status["retention_seconds"] == 604800
                    and status["budget_bytes"] == 256 << 20,
                    "explicit UI activation did not preserve fixed limits and default-off capture",
                )
                checks["explicit_activation_updates_panel_and_topbar"] = True

                panel.get_by_label("仅删除已达到保留期限的独立密文", exact=False).check()
                panel.get_by_role("button", name="清理到期原文", exact=True).click()
                expect(panel.get_by_text("已清理 0 条到期密文，释放 0 B。", exact=False)).to_be_visible()
                checks["expired_only_purge_requires_confirmation"] = True
                panel.screenshot(
                    path=str(args.out_dir / "raw-content-panel-desktop.png"),
                    animations="disabled",
                )
                page.screenshot(
                    path=str(args.out_dir / "raw-content-ready-desktop.png"),
                    full_page=True,
                    animations="disabled",
                )

                page.reload()
                expect(panel.get_by_text("原文仓已启用，默认采集仍为关闭。", exact=True)).to_be_visible()
                expect(page.get_by_text("原文仓已启用 · 按任务授权", exact=True)).to_be_visible()
                checks["signed_activation_survives_reload"] = True

                corrupt_status = {
                    **status,
                    "default_capture": True,
                }
                page.route(
                    "**/v1/raw-task-content/status",
                    lambda route: route.fulfill(json=corrupt_status),
                )
                panel.get_by_role("button", name="刷新状态", exact=True).click()
                expect(panel.get_by_role("alert")).to_contain_text("原文记录状态响应不完整")
                page.evaluate("window.dispatchEvent(new Event('siq:raw-content-status-changed'))")
                expect(page.get_by_text("原文状态不可用", exact=True)).to_be_visible()
                fixture.require(
                    panel.get_by_text("原文仓已启用，默认采集仍为关闭。", exact=True).count()
                    == 0,
                    "invalid status response left stale ready controls visible",
                )
                checks["invalid_contract_fails_closed_without_stale_ready_state"] = True
                page.unroute("**/v1/raw-task-content/status")
                panel.get_by_role("button", name="刷新状态", exact=True).click()
                page.evaluate("window.dispatchEvent(new Event('siq:raw-content-status-changed'))")
                expect(page.get_by_text("原文仓已启用 · 按任务授权", exact=True)).to_be_visible()

                storage = page.evaluate(
                    "JSON.stringify({local: {...localStorage}, session: {...sessionStorage}})"
                ).lower()
                fixture.require(
                    "local-raw-task-content" not in storage
                    and "raw_task_content" not in storage
                    and "retention_seconds" not in storage
                    and "budget_bytes" not in storage,
                    "raw-content authority or status was written to browser storage",
                )
                checks["raw_authority_is_not_browser_persisted"] = True

                page.set_viewport_size({"width": 390, "height": 844})
                page.wait_for_function(
                    "document.querySelector('.sidebar').getBoundingClientRect().right <= 0"
                )
                panel.scroll_into_view_if_needed()
                fixture.require(
                    page.evaluate("document.documentElement.scrollWidth <= innerWidth"),
                    "raw-content settings cause mobile horizontal overflow",
                )
                page.screenshot(
                    path=str(args.out_dir / "raw-content-ready-mobile.png"),
                    full_page=True,
                    animations="disabled",
                )
                checks["mobile_has_no_horizontal_overflow"] = True
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
