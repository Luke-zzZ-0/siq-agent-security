export type ActivityView = 'tasks' | 'unassigned';
export interface ActivityBinding {
  chain_id: string; platform: string; session_id: string; agent_id: string;
  task_id: string; intent_id: string; intent_digest: string;
}
export interface TaskActivityItem {
  activity_id: string; attribution: 'bound' | 'unknown'; binding: ActivityBinding | null;
  receipt_count: number; first_seq: number; last_seq: number;
}
export interface TaskActivityPage {
  schema_version: 'local-task-activities/v1'; snapshot: string; view: ActivityView;
  offset: number; total: number; next_offset: number | null; prefix_valid: true;
  history_integrity: 'verified' | 'unknown' | 'failed'; evidence_freshness: 'unknown';
  items: TaskActivityItem[];
}
export interface ActivityFilters {
  platform: string; agent_id: string; session_id: string; task_id: string; q: string;
}
export interface TaskActivitySearch extends Omit<TaskActivityPage, 'schema_version'> {
  schema_version: 'local-task-activity-search/v1'; filters: ActivityFilters;
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const integer = (v: unknown): v is number => Number.isSafeInteger(v) && (v as number) >= 0;
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
export function isTaskActivityPage(value: unknown, view: ActivityView, offset: number, snapshot?: string, pageSize = 50): value is TaskActivityPage {
  if (!object(value) || value.schema_version !== 'local-task-activities/v1' || value.view !== view || value.offset !== offset ||
    !hash(value.snapshot) || (snapshot && snapshot !== value.snapshot) || value.prefix_valid !== true ||
    !['verified', 'unknown', 'failed'].includes(String(value.history_integrity)) || value.evidence_freshness !== 'unknown' ||
    !integer(value.total) || !Array.isArray(value.items) || value.items.length > pageSize) return false;
  const end = Math.min(offset + pageSize, value.total);
  if (value.items.length !== Math.max(0, end - offset) || value.next_offset !== (end < value.total ? end : null)) return false;
  const seen = new Set<string>();
  for (const item of value.items) {
    if (!object(item) || !hash(item.activity_id) || seen.has(item.activity_id) || !integer(item.receipt_count) || item.receipt_count < 1 ||
      !integer(item.first_seq) || !integer(item.last_seq) || item.last_seq < item.first_seq) return false;
    seen.add(item.activity_id);
    if (view === 'unassigned') {
      if (item.attribution !== 'unknown' || item.binding !== null || item.receipt_count !== 1 || item.first_seq !== item.last_seq) return false;
    } else {
      if (item.attribution !== 'bound' || !object(item.binding)) return false;
      for (const field of ['chain_id', 'platform', 'session_id', 'agent_id', 'task_id', 'intent_id', 'intent_digest']) {
        if (typeof item.binding[field] !== 'string' || !item.binding[field]) return false;
      }
    }
  }
  return true;
}

const filterKeys = ['platform', 'agent_id', 'session_id', 'task_id', 'q'] as const;
export function validActivityFilter(value: string): boolean {
  return new TextEncoder().encode(value).length <= 1024 && [...value].length <= 256 && !/[\u0000-\u001f\u007f-\u009f]/.test(value);
}
export function isTaskActivitySearch(value: unknown, view: ActivityView, offset: number, filters: ActivityFilters, snapshot?: string, pageSize = 50): value is TaskActivitySearch {
  if (!object(value) || value.schema_version !== 'local-task-activity-search/v1' || !object(value.filters)) return false;
  const responseFilters = value.filters;
  if (Object.keys(responseFilters).length !== filterKeys.length || !filterKeys.every((key) => responseFilters[key] === filters[key])) return false;
  const page = { ...value, schema_version: 'local-task-activities/v1' };
  return isTaskActivityPage(page, view, offset, snapshot, pageSize);
}

export interface ActivityReceipt {
  receipt_id: string; seq: number; hash: string; issued_at: string;
  platform: string; session_id: string; agent_id: string | null;
  action: string; reason: string; tool: string; record_type: string;
  action_id: string; decision_receipt_id: string; matched_grant_id: string | null;
}
export interface TaskActivityDetail extends Omit<TaskActivityPage, 'schema_version' | 'items'> {
  schema_version: 'local-task-activity-detail/v1'; activity: TaskActivityItem; receipts: ActivityReceipt[];
}
export function isTaskActivityDetail(value: unknown, id: string, view: ActivityView, offset: number, snapshot?: string, pageSize = 50): value is TaskActivityDetail {
  if (!object(value) || value.schema_version !== 'local-task-activity-detail/v1' || value.offset !== offset || !integer(value.total) ||
    !object(value.activity) || value.activity.activity_id !== id || value.activity.receipt_count !== value.total || !Array.isArray(value.receipts)) return false;
  const summary = { ...value, schema_version: 'local-task-activities/v1', items: [value.activity], offset: 0, total: 1, next_offset: null };
  if (!isTaskActivityPage(summary, view, 0, snapshot, 1)) return false;
  const end = Math.min(offset + pageSize, value.total);
  if (value.receipts.length !== Math.max(0, end - offset) || value.next_offset !== (end < value.total ? end : null)) return false;
  const activity = summary.items[0];
  let previous = -1;
  for (const rc of value.receipts) {
    if (!object(rc) || !integer(rc.seq) || rc.seq <= previous || rc.seq < activity.first_seq || rc.seq > activity.last_seq || !hash(rc.hash)) return false;
    previous = rc.seq;
    for (const field of ['receipt_id', 'issued_at', 'platform', 'session_id', 'action', 'reason', 'tool', 'record_type', 'action_id', 'decision_receipt_id']) {
      if (typeof rc[field] !== 'string') return false;
    }
    for (const field of ['agent_id', 'matched_grant_id']) if (rc[field] !== null && typeof rc[field] !== 'string') return false;
    if ('params' in rc || 'params_excerpt' in rc || 'token' in rc) return false;
    if (activity.binding && (rc.platform !== activity.binding.platform || rc.session_id !== activity.binding.session_id || rc.agent_id !== activity.binding.agent_id)) return false;
  }
  return true;
}
