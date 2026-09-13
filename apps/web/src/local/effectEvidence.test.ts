import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { readEffectEvidence } from './effectEvidence';
const evidence = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/effect-evidence.sample.json', import.meta.url), 'utf8'));
const record = { schema_version: 'effect-evidence-record/v1', task_id: 'task', evidence, finding_code: '', request_digest: 'a'.repeat(64), signature: 'b'.repeat(128), signing_schema: 'local_canonical/v1' };
describe('effect metadata projection', () => {
  it('projects only metadata from the shared evidence example', () => {
    const got = readEffectEvidence({ ...record, file_observation: { content: 'PRIVATE' } }, evidence.effect_evidence_id, 'task');
    expect(got?.sourceId).toBe('observer-1');
    expect(JSON.stringify(got)).not.toContain('PRIVATE');
    expect(JSON.stringify(got)).not.toContain('signature');
  });
  it('rejects substituted task/evidence and malformed metadata', () => {
    expect(readEffectEvidence(record, 'other', 'task')).toBeNull();
    expect(readEffectEvidence(record, evidence.effect_evidence_id, 'other')).toBeNull();
    for (const patch of [{ source: { ...evidence.source, independence: 'verified' } }, { coverage: 'all' }, { observed_at: 'invalid' }, { resource_ref: '/private/file' }, { signature: '' }]) {
      expect(readEffectEvidence({ ...record, evidence: { ...evidence, ...patch } }, evidence.effect_evidence_id, 'task')).toBeNull();
    }
  });
});
