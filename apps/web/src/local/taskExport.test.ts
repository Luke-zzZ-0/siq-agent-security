import { readFileSync } from 'node:fs';
import { expect, it } from 'vitest';
import { isTaskExport } from './taskExport';
import type { TaskActivityItem } from './taskActivities';
const data = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-export.json', import.meta.url), 'utf8'));
const activity: TaskActivityItem = { activity_id: data.activity_id, attribution: 'bound', binding: null, receipt_count: 2, first_seq: 0, last_seq: 2 };
it('validates the shared signed Go export against the selected complete activity', () => {
  expect(isTaskExport(data, activity, data.snapshot)).toBe(true);
  expect(isTaskExport(data, { ...activity, attribution: 'unknown' }, data.snapshot)).toBe(false);
  expect(isTaskExport(data, activity, '0'.repeat(64))).toBe(false);
  for (const patch of [{ signature: '' }, { params: 'secret' }, { receipts: [data.receipts[0]] }, { prefix_valid: false }, { source_last_seq: 0 }]) expect(isTaskExport({ ...data, ...patch }, activity, data.snapshot)).toBe(false);
});
it('rejects private extensions and mixed or repeated receipt rows', () => {
  for (const patch of [{ params_excerpt: 'secret' }, { reason: 'secret' }, { seq: 0 }, { seq: 3 }, { action: 'completed' }]) {
    expect(isTaskExport({ ...data, receipts: [data.receipts[0], { ...data.receipts[1], ...patch }] }, activity, data.snapshot)).toBe(false);
  }
});
