import { readFileSync } from 'node:fs';
import { expect, it } from 'vitest';
import { isActivitySources } from './taskSources';
import type { TaskActivityDetail } from './taskActivities';
const source = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-sources.json', import.meta.url), 'utf8'));
const base = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-detail.json', import.meta.url), 'utf8'));
const detail: TaskActivityDetail = { ...base, snapshot: source.snapshot, activity: { ...base.activity, activity_id: source.activity_id }, receipts: [{ ...base.receipts[0], hash: source.items[0].receipt_hash, matched_grant_id: source.items[0].source.grant_id }] };
it('accepts shared Go sources only against the same page and receipt identity', () => {
  expect(isActivitySources(source, detail)).toBe(true);
  for (const patch of [{ snapshot: '0'.repeat(64) }, { total: 4 }, { items: [] }, { token: 'secret' }]) expect(isActivitySources({ ...source, ...patch }, detail)).toBe(false);
  for (const patch of [{ seq: 2 }, { receipt_hash: 'f'.repeat(64) }, { status: 'completed' }, { source: null }]) expect(isActivitySources({ ...source, items: [{ ...source.items[0], ...patch }] }, detail)).toBe(false);
});
it('rejects substituted grants, private fields and unavailable source claims', () => {
  for (const patch of [{ grant_id: 'other' }, { params: 'secret' }, { skill_name: 'bad\nname' }, { content_hash: '' }, { admission_id: 'adm-si-invalid' }]) {
    expect(isActivitySources({ ...source, items: [{ ...source.items[0], source: { ...source.items[0].source, ...patch } }] }, detail)).toBe(false);
  }
  expect(isActivitySources({ ...source, items: [{ ...source.items[0], status: 'unavailable', source: null }] }, detail)).toBe(true);
  expect(isActivitySources({ ...source, items: [{ ...source.items[0], status: 'unavailable' }] }, detail)).toBe(false);
});
it('preserves unknown attribution and separate imported digest domains', () => {
  const unknown = { ...source, view: 'unassigned', items: [{ ...source.items[0], status: 'unattributed', source: null }] };
  expect(isActivitySources(unknown, { ...detail, view: 'unassigned' })).toBe(true);
  expect(isActivitySources(unknown, detail)).toBe(false);
  const imported = { ...source.items[0].source, declared_version: null, admission_id: 'adm-si-' + 'd'.repeat(64), import: { schema_version: 'local-skill-import-permission-source/v1', import_id: 'si-' + '1'.repeat(32), artifact_digest: 'b'.repeat(64), analysis_sha256: 'c'.repeat(64) } };
  expect(isActivitySources({ ...source, items: [{ ...source.items[0], source: imported }] }, detail)).toBe(true);
  expect(isActivitySources({ ...source, items: [{ ...source.items[0], source: { ...imported, import: { ...imported.import, artifact_digest: '' } } }] }, detail)).toBe(false);
});
