import { readFileSync } from 'node:fs';
import { expect, it } from 'vitest';
import type { TaskActivityItem } from './taskActivities';
import { isTaskTraceExport } from './taskTraceExport';

const data = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-trace-export.json', import.meta.url), 'utf8'));
const activity: TaskActivityItem = { activity_id: data.activity_id, attribution: 'bound', binding: null, receipt_count: 1, first_seq: 0, last_seq: 0 };

it('validates the Go trace fixture and its redacted joins', () => {
  expect(isTaskTraceExport(data, activity, data.snapshot)).toBe(true);
  expect(isTaskTraceExport(data, { ...activity, attribution: 'unknown' }, data.snapshot)).toBe(false);
  for (const patch of [{ incomplete: false }, { effects: [] }, { sources: [] }, { params: 'PRIVATE' }, { attestation_scope: 'raw_task_trace' }]) {
    expect(isTaskTraceExport({ ...data, ...patch }, activity, data.snapshot)).toBe(false);
  }
});

it('rejects source, completion and evidence relationship changes', () => {
  const source = data.sources[0];
  const completion = data.completion;
  const requirement = completion.requirements[0];
  const effect = data.effects[0];
  expect(isTaskTraceExport({ ...data, sources: [{ ...source, source: null }] }, activity, data.snapshot)).toBe(false);
  expect(isTaskTraceExport({ ...data, completion: { ...completion, requirements: [{ ...requirement, evidence_refs: [] }] } }, activity, data.snapshot)).toBe(false);
  expect(isTaskTraceExport({ ...data, effects: [{ ...effect, evidence_ref: `sha256:${'0'.repeat(64)}` }] }, activity, data.snapshot)).toBe(false);
  expect(isTaskTraceExport({ ...data, effects: [{ ...effect, observation: 'PRIVATE' }] }, activity, data.snapshot)).toBe(false);
});
