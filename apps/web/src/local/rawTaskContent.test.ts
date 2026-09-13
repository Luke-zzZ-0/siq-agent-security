import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { formatBytes, formatRetention, isRawContentActivation, isRawContentPurgeResult, isRawContentStatus } from './rawTaskContent';

const fixture = (name: string) => JSON.parse(readFileSync(new URL(`../../../agentshield/testdata/contracts/${name}.json`, import.meta.url), 'utf8'));

describe('raw task content privacy contracts', () => {
  it('accepts fixed status and activation samples without broadening ready semantics', () => {
    const status = fixture('local-raw-task-content-status');
    const activation = fixture('local-raw-task-content-activation');
    expect(isRawContentStatus(status)).toBe(true);
    expect(isRawContentActivation(activation)).toBe(true);
    expect(isRawContentStatus({ ...status, default_capture: true })).toBe(false);
    expect(isRawContentStatus({ ...status, status: 'disabled' })).toBe(false);
    expect(isRawContentStatus({ ...status, token: 'private' })).toBe(false);
    expect(isRawContentActivation({ ...activation, signature: 'A'.repeat(128) })).toBe(false);
    expect(isRawContentActivation({ ...activation, key_fingerprint: 'raw-key' })).toBe(false);
  });

  it('requires null limits for disabled/error and bounded limits for ready', () => {
    const status = fixture('local-raw-task-content-status');
    for (const state of ['disabled', 'error']) {
      expect(isRawContentStatus({ ...status, status: state, retention_seconds: null, budget_bytes: null, activated_at: null })).toBe(true);
    }
    expect(isRawContentStatus({ ...status, retention_seconds: 3599 })).toBe(false);
    expect(isRawContentStatus({ ...status, budget_bytes: 1073741825 })).toBe(false);
    expect(isRawContentStatus({ ...status, activated_at: 'invalid' })).toBe(false);
  });

  it('validates purge summaries and formats bounded settings', () => {
    const purge = fixture('local-raw-task-content-purge-result');
    expect(isRawContentPurgeResult(purge)).toBe(true);
    expect(isRawContentPurgeResult({ ...purge, deleted_records: 4097 })).toBe(false);
    expect(isRawContentPurgeResult({ ...purge, plaintext: 'private' })).toBe(false);
    expect(formatBytes(64 << 20)).toBe('64 MiB');
    expect(formatRetention(86400)).toBe('1 天');
  });
});
