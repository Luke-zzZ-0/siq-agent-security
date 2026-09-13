import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { isTaskActivityPage, isTaskActivitySearch, validActivityFilter } from './taskActivities';
const sample = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activities.json', import.meta.url), 'utf8'));
const search = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-search.json', import.meta.url), 'utf8'));
describe('task activities response boundaries', () => {
  it('accepts the shared Go page only for its requested scope', () => {
    expect(isTaskActivityPage(sample, 'tasks', 0, undefined, 1)).toBe(true);
    expect(isTaskActivityPage(sample, 'unassigned', 0, undefined, 1)).toBe(false);
    expect(isTaskActivityPage(sample, 'tasks', 1, undefined, 1)).toBe(false);
    expect(isTaskActivityPage(sample, 'tasks', 0, 'f'.repeat(64), 1)).toBe(false);
  });
  it('rejects false integrity, broken pagination and invented attribution', () => {
    for (const patch of [{ prefix_valid: false }, { next_offset: null }, { items: [] }, { evidence_freshness: 'verified' }, { total: -1 }]) {
      expect(isTaskActivityPage({ ...sample, ...patch }, 'tasks', 0, undefined, 1)).toBe(false);
    }
    expect(isTaskActivityPage({ ...sample, items: [{ ...sample.items[0], binding: null }] }, 'tasks', 0, undefined, 1)).toBe(false);
    const unknown = { ...sample, view: 'unassigned', total: 1, next_offset: null, items: [{ ...sample.items[0], attribution: 'unknown', binding: null, receipt_count: 1, last_seq: sample.items[0].first_seq }] };
    expect(isTaskActivityPage(unknown, 'unassigned', 0)).toBe(true);
    expect(isTaskActivityPage({ ...unknown, items: [{ ...unknown.items[0], receipt_count: 2 }] }, 'unassigned', 0)).toBe(false);
  });
});

describe('task activity search boundaries', () => {
  it('accepts the shared Go result only for the complete URL scope', () => {
    expect(isTaskActivitySearch(search, 'tasks', 0, search.filters, undefined, 1)).toBe(true);
    expect(isTaskActivitySearch(search, 'unassigned', 0, search.filters, undefined, 1)).toBe(false);
    expect(isTaskActivitySearch(search, 'tasks', 0, { ...search.filters, q: 'other' }, undefined, 1)).toBe(false);
    expect(isTaskActivitySearch(search, 'tasks', 0, search.filters, 'f'.repeat(64), 1)).toBe(false);
  });
  it('rejects invented filters and applies the server character boundary', () => {
    expect(isTaskActivitySearch({ ...search, filters: { ...search.filters, params: 'secret' } }, 'tasks', 0, search.filters, undefined, 1)).toBe(false);
    expect(validActivityFilter('报'.repeat(256))).toBe(true);
    expect(validActivityFilter('报'.repeat(257))).toBe(false);
    expect(validActivityFilter('bad\nvalue')).toBe(false);
    expect(validActivityFilter('')).toBe(true);
  });
});

import { isTaskActivityDetail } from './taskActivities';
const detail = JSON.parse(readFileSync(new URL('../../../agentshield/testdata/contracts/local-task-activity-detail.json', import.meta.url), 'utf8'));
describe('activity detail boundaries', () => {
  it('accepts only the requested activity and scoped receipts', () => {
    expect(isTaskActivityDetail(detail, detail.activity.activity_id, 'tasks', 0, undefined, 1)).toBe(true);
    expect(isTaskActivityDetail(detail, 'f'.repeat(64), 'tasks', 0, undefined, 1)).toBe(false);
    for (const patch of [{ session_id: 'other' }, { agent_id: 'other' }, { platform: 'other' }, { seq: 3 }, { params_excerpt: 'secret' }]) {
      expect(isTaskActivityDetail({ ...detail, receipts: [{ ...detail.receipts[0], ...patch }] }, detail.activity.activity_id, 'tasks', 0, undefined, 1)).toBe(false);
    }
    expect(isTaskActivityDetail({ ...detail, total: 3 }, detail.activity.activity_id, 'tasks', 0, undefined, 1)).toBe(false);
    expect(isTaskActivityDetail({ ...detail, next_offset: null }, detail.activity.activity_id, 'tasks', 0, undefined, 1)).toBe(false);
  });
});
