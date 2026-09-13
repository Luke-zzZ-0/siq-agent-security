import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import {
  isRawContentGrant,
  isRawContentRecordContent,
  isRawContentRecordDeleted,
  rawContentTaskRef,
  readRawContentGrants,
  readRawContentRecords,
} from './rawTaskContentManagement';

const fixture = (name: string) => JSON.parse(readFileSync(
  new URL(`../../../agentshield/testdata/contracts/${name}.json`, import.meta.url),
  'utf8',
));

describe('raw task content management contracts', () => {
  it('computes the same task references used by the Go fixtures', async () => {
    expect(await rawContentTaskRef('fixture-task')).toBe(fixture('local-raw-task-content-grant').task_ref);
    expect(await rawContentTaskRef('fixture-runtime-task')).toBe(fixture('local-raw-task-content-records').items[0].task_ref);
  });

  it('validates grants, terminal revocations and stable collections', () => {
    const grant = fixture('local-raw-task-content-grant');
    const request = fixture('local-raw-task-content-grant-create');
    expect(isRawContentGrant(grant, grant.task_ref, {
      taskId: request.task_id,
      kinds: request.kinds,
      actorId: request.actor_id,
      durationSeconds: request.duration_seconds,
      retentionSeconds: request.retention_seconds,
      maxPlaintextBytes: request.max_plaintext_bytes,
    })).toBe(true);
    expect(readRawContentGrants(fixture('local-raw-task-content-grants'))).toHaveLength(1);
    expect(readRawContentGrants({ ...fixture('local-raw-task-content-grants'), token: 'private' })).toBeNull();
    const view = fixture('local-raw-task-content-grant-view');
    expect(readRawContentGrants({
      schema_version: 'local-raw-task-content-grants/v1',
      items: [view, view],
    })).toBeNull();
    expect(readRawContentGrants({
      schema_version: 'local-raw-task-content-grants/v1',
      items: [{ ...view, status: 'active' }],
    })).toBeNull();
  });

  it('binds record lists and one-time plaintext responses to the selected task and record', () => {
    const records = fixture('local-raw-task-content-records');
    const record = records.items[0];
    expect(readRawContentRecords(records, record.task_ref)).toHaveLength(1);
    expect(readRawContentRecords(records, `sha256:${'0'.repeat(64)}`)).toBeNull();
    expect(readRawContentRecords({ ...records, items: [record, record] }, record.task_ref)).toBeNull();
    const content = fixture('local-raw-task-content-record-content');
    expect(isRawContentRecordContent(content, record)).toBe(true);
    expect(isRawContentRecordContent({ ...content, contains_plaintext: false }, record)).toBe(false);
    expect(isRawContentRecordContent({ ...content, fields: [{ path: '/prompt', value: 'changed' }] }, record)).toBe(true);
    expect(isRawContentRecordContent({ ...content, record: { ...record, record_id: `raw-${'0'.repeat(32)}` } }, record)).toBe(false);
  });

  it('requires an exact deletion acknowledgement', () => {
    const result = fixture('local-raw-task-content-record-deleted');
    expect(isRawContentRecordDeleted(result, result.record_id)).toBe(true);
    expect(isRawContentRecordDeleted(result, `raw-${'0'.repeat(32)}`)).toBe(false);
    expect(isRawContentRecordDeleted({ ...result, task_id: 'private' }, result.record_id)).toBe(false);
  });
});
