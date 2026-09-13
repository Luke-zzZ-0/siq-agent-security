#!/usr/bin/env python3
"""Exercise embedded update-check UI with an isolated daemon and explicit response fixtures."""
import argparse
import importlib.util
import json
import re
import shutil
import tempfile
from pathlib import Path

from playwright.sync_api import expect, sync_playwright

REPO = Path(__file__).resolve().parents[2]
loader = importlib.util.spec_from_file_location('daemon_fixture', REPO / 'scripts/validate-intent-v2-hermes.py')
fixture = importlib.util.module_from_spec(loader)
loader.loader.exec_module(fixture)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', type=Path, required=True)
    parser.add_argument('--out-dir', type=Path, required=True)
    args = parser.parse_args()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    samples = REPO / 'apps/agentshield/testdata/contracts'
    catalog = json.loads((samples / 'local-skill-install-catalog.v1.sample.json').read_text())
    rec = catalog['items'][0]
    ident = rec['install_id']
    inspection = json.loads((samples / 'local-skill-install-inspection.v1.sample.json').read_text())
    inspection['record'] = rec
    removal = dict(schema_version='local-skill-install-removal-view/v1', record=rec, claim=None, result=None,
                   grant=dict(grant_id=rec['plan']['grant_id'], signature='a'*128, status='approved', platform='hermes',
                              subject=dict(type='agent_instance', id=rec['plan']['instance_id'].replace('hi-', 'hri-')), facts=[]),
                   state_revision=1, status='not_requested', will_revoke_grant=True, retained_install_id='', binding_signature='')
    result = json.loads((samples / 'local-skill-update-check-result.json').read_text()) | {'install_id': ident}
    with tempfile.TemporaryDirectory(prefix='siq-update-check-browser-') as temp:
        root = Path(temp)
        args.installer_managed_profile = True
        args.hermes_cli = root / 'unavailable-hermes-cli'
        h = fixture.Harness(root, args)
        shutil.copy2(args.binary.resolve(), h.binary)
        try:
            h.start()
            pairing = h.command([str(h.binary), 'pair', '--port', h.endpoint.rsplit(':', 1)[1]])
            code = re.search(r'\b[0-9a-f]{4}(?:-[0-9a-f]{4}){3}\b', pairing).group()
            with sync_playwright() as pw:
                browser = pw.chromium.launch(headless=True)
                page = browser.new_page(viewport={'width': 1440, 'height': 1000}, locale='zh-CN')
                errors = []
                page.on('pageerror', lambda e: errors.append(str(e)))
                page.route('**/v1/skill-installations/operations', lambda route: route.fulfill(json=catalog))
                page.route('**/inspection', lambda route: route.fulfill(json=inspection))
                page.route('**/removal', lambda route: route.fulfill(json=removal))
                mode = 'changed'
                pending = []
                requests = []

                def check(route):
                    body = route.request.post_data_json
                    assert body['remote_url'] == 'https://downloads.example.com/skill.zip'
                    assert body['schema_version'] == 'local-skill-update-check/v1'
                    requests.append(body['schema_version'])
                    if mode == 'pending':
                        pending.append(route)
                    elif mode in ('unavailable', 'unsupported'):
                        route.fulfill(status=503, json={'error': 'skill_install_unavailable' if mode == 'unavailable' else 'skill_update_source_unavailable'})
                    elif mode == 'same':
                        route.fulfill(json=result | {'status': 'up_to_date', 'content_changes': [], 'content_changes_total': 0, 'requires_confirmation': False})
                    elif mode == 'forged':
                        route.fulfill(json=result | {'install_id': 'sin-' + 'f'*64})
                    else:
                        route.fulfill(json=result)

                page.route('**/update-check', check)
                page.goto(h.endpoint + '/installed-skills')
                page.get_by_label('配对码', exact=True).fill(code)
                page.get_by_role('button', name='建立管理会话', exact=True).click()
                panel = page.get_by_role('region', name='检查 Skill 新版')
                expect(panel).to_be_visible()
                panel.get_by_label('原 HTTPS ZIP 下载链接', exact=True).fill('https://downloads.example.com/skill.zip')
                button = panel.get_by_role('button', name='检查新版', exact=True)
                button.click()
                expect(panel.get_by_text('发现 1 项内容变化，需要你确认后更新。', exact=True)).to_be_visible()
                expect(panel.get_by_role('link', name='审阅候选并更新')).to_have_attribute('href', '/skill-updates?install_id=' + ident)
                page.screenshot(path=str(args.out_dir / 'desktop.png'), full_page=True)
                mode = 'same'
                button.click()
                expect(panel.get_by_text('上游内容与安装记录一致。', exact=True)).to_be_visible()
                for mode, message in [('unavailable', '暂时无法获取上游'), ('unsupported', 'Git 来源暂不支持安全获取'), ('forged', '未能确认新版检查结果')]:
                    button.click()
                    expect(panel.get_by_role('alert')).to_contain_text(message)
                    expect(panel.get_by_text('上游内容与安装记录一致。', exact=True)).to_have_count(0)
                mode = 'pending'
                button.click()
                expect(panel.get_by_text('正在获取上游并比较内容…', exact=True)).to_be_visible()
                panel.get_by_role('button', name='取消检查').click()
                expect(panel.get_by_role('alert')).to_have_text('检查已取消。')
                for route in pending:
                    try:
                        route.fulfill(json=result)
                    except Exception:
                        pass
                expect(panel.get_by_text('发现 1 项内容变化，需要你确认后更新。', exact=True)).to_have_count(0)
                mode = 'changed'
                page.set_viewport_size({'width': 390, 'height': 844})
                button.click()
                expect(panel.get_by_text('发现 1 项内容变化，需要你确认后更新。', exact=True)).to_be_visible()
                panel.screenshot(path=str(args.out_dir / 'mobile.png'))
                assert not errors, errors
                assert page.evaluate('document.documentElement.scrollWidth <= window.innerWidth'), 'horizontal overflow'
                browser.close()
                (args.out_dir / 'verification.json').write_text(json.dumps({'status': 'passed', 'fixture_response_checks': ['changed', 'same', 'unavailable', 'unsupported', 'wrong_install_id', 'cancel_discards_late_response', 'mobile'], 'request_count': len(requests), 'page_errors': errors, 'native_upstream_acceptance': False}, indent=2) + '\n')
        finally:
            h.stop()


if __name__ == '__main__':
    main()
