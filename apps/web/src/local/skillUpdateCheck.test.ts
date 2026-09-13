import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { isSkillUpdateCheckResult, updateCheckErrorText } from './skillUpdateCheck';
const sample = () => JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-skill-update-check-result.json', import.meta.url), 'utf8'));
describe('upstream checks never authorize an update', () => {
  it('accepts the Go contract sample and binds it to the installation', () => {
    const v = sample(); expect(isSkillUpdateCheckResult(v, v.install_id)).toBe(true);
    expect(isSkillUpdateCheckResult(v, 'sin-' + 'e'.repeat(64))).toBe(false);
    expect(isSkillUpdateCheckResult({ ...v, status: 'up_to_date', content_changes: [], content_changes_total: 0, requires_confirmation: false }, v.install_id)).toBe(true);
  });
  it('rejects inconsistent, oversized or malformed evidence', () => {
    const v = sample();
    for (const patch of [{ requires_confirmation: false }, { checked_at: 'now' }, { content_changes_total: 4 }, { content_changes_truncated: true },
      { content_changes: [{ ...v.content_changes[0], before: null, after: null }] }, { permission_comparison: 'approved' }, { raw_url: 'private' },
      { upstream_commit_sha: 'a'.repeat(40) }, { content_changes: Array(201).fill(v.content_changes[0]) }]) expect(isSkillUpdateCheckResult({ ...v, ...patch }, v.install_id)).toBe(false);
    expect(isSkillUpdateCheckResult({ ...v, content_changes: Array(200).fill(v.content_changes[0]), content_changes_total: 201, content_changes_truncated: true }, v.install_id)).toBe(true);
  });
  it('does not echo raw transport diagnostics and distinguishes unavailable from changed', () => {
    expect(updateCheckErrorText(new Error('private-token'))).not.toContain('private-token');
    expect(updateCheckErrorText(new Error('skill_install_unavailable'))).toContain('未完成检查');
    expect(updateCheckErrorText(new Error('skill_install_changed'))).toContain('不一致');
  });
});
