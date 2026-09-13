import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { isActivityCompletion } from './taskCompletion';
const sample = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-completion.json', import.meta.url), 'utf8'));
const validate = (value: unknown) => isActivityCompletion(value, sample.activity, sample.snapshot, 'tasks');
describe('activity effect evidence presentation', () => {
  it('accepts the Go unknown fixture but never upgrades empty evidence', () => {
    expect(validate(sample)).toBe(true);
    expect(validate({ ...sample, result: { ...sample.result, status: 'verified' } })).toBe(false);
    expect(validate({ ...sample, reason_code: 'intent_missing', result: null })).toBe(true);
    expect(validate({ ...sample, reason_code: 'attribution_unknown', result: null })).toBe(false);
  });
  it('requires scoped, populated evidence for a verified result', () => {
    const item = { requirement_id: 'write', status: 'verified', reason_code: 'effect_verified', evidence_ids: ['evidence-1'] };
    const good = { ...sample, result: { ...sample.result, status: 'verified', reason_code: 'effects_verified', requirements: [item] } };
    expect(validate(good)).toBe(true);
    for (const patch of [{ task_id: 'other' }, { status: 'completed' }, { incident_ids: ['incident'] }, { requirements: [{ ...item, evidence_ids: [] }] }, { requirements: [{ ...item, status: 'unknown' }] }]) {
      expect(validate({ ...good, result: { ...good.result, ...patch } })).toBe(false);
    }
    expect(validate({ ...good, snapshot: 'f'.repeat(64) })).toBe(false);
    expect(validate({ ...good, activity: { ...good.activity, binding: { ...good.activity.binding, session_id: 'other' } } })).toBe(false);
  });
});
