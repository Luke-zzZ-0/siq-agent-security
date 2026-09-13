import { isTaskActivityPage, type ActivityView, type TaskActivityItem } from './taskActivities';
export type CompletionStatus = 'verified' | 'incomplete' | 'conflicting' | 'unknown';
export interface CompletionItem { requirement_id: string; status: CompletionStatus; reason_code: string; evidence_ids: string[] }
export interface CompletionResult { schema_version: 'completion-status/v1'; task_id: string; status: CompletionStatus; reason_code: string; requirements: CompletionItem[]; incident_ids: string[] }
export interface ActivityCompletion {
  schema_version: 'local-task-activity-completion/v1'; activity: TaskActivityItem; snapshot: string; evaluated_at: string;
  reason_code: 'evaluated' | 'attribution_unknown' | 'intent_missing'; result: CompletionResult | null;
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const text = (v: unknown): v is string => typeof v === 'string' && v.length > 0;
const status = (v: unknown): v is CompletionStatus => ['verified', 'incomplete', 'conflicting', 'unknown'].includes(String(v));
const ids = (v: unknown): v is string[] => Array.isArray(v) && v.length <= 8192 && v.every(text) && new Set(v).size === v.length;
export function isActivityCompletion(value: unknown, activity: TaskActivityItem, snapshot: string, view: ActivityView): value is ActivityCompletion {
  if (!object(value) || value.schema_version !== 'local-task-activity-completion/v1' || value.snapshot !== snapshot ||
    !text(value.evaluated_at) || !Number.isFinite(Date.parse(value.evaluated_at))) return false;
  const summary = { schema_version: 'local-task-activities/v1', view, offset: 0, total: 1, next_offset: null, snapshot,
    prefix_valid: true, history_integrity: 'unknown', evidence_freshness: 'unknown', items: [value.activity] };
  if (!isTaskActivityPage(summary, view, 0, snapshot, 1)) return false;
  const got = summary.items[0];
  if (got.activity_id !== activity.activity_id || got.attribution !== activity.attribution || got.receipt_count !== activity.receipt_count ||
    got.first_seq !== activity.first_seq || got.last_seq !== activity.last_seq) return false;
  for (const field of ['chain_id', 'platform', 'session_id', 'agent_id', 'task_id', 'intent_id', 'intent_digest'] as const) {
    if (got.binding?.[field] !== activity.binding?.[field]) return false;
  }
  if (value.reason_code === 'attribution_unknown') return activity.binding === null && value.result === null;
  if (value.reason_code === 'intent_missing') return activity.binding !== null && value.result === null;
  const result = value.result;
  if (value.reason_code !== 'evaluated' || !activity.binding || !object(result) || result.schema_version !== 'completion-status/v1' ||
    result.task_id !== activity.binding.task_id || !status(result.status) || !text(result.reason_code) || !ids(result.incident_ids) ||
    !Array.isArray(result.requirements) || result.requirements.length > 128) return false;
  const seen = new Set<string>();
  for (const item of result.requirements) {
    if (!object(item) || !text(item.requirement_id) || seen.has(item.requirement_id) || !status(item.status) || !text(item.reason_code) || !ids(item.evidence_ids)) return false;
    seen.add(item.requirement_id);
    if (item.status === 'verified' && item.evidence_ids.length === 0) return false;
  }
  if (result.status === 'verified' && (result.requirements.length === 0 || result.incident_ids.length > 0 || result.requirements.some((item) => item.status !== 'verified'))) return false;
  return true;
}
export const completionLabel: Record<CompletionStatus, string> = { verified: '效果已核验', incomplete: '效果尚未完成', conflicting: '效果证据存在冲突', unknown: '效果仍未知' };
export function completionReason(reason: string): string {
  return ({ not_required: '未定义可核验的效果要求。', effect_evidence_missing: '缺少效果证据。', effect_evidence_insufficient: '证据独立性或覆盖范围不足。', effect_failed: '观测到执行失败。', effect_evidence_conflicting: '证据之间存在冲突。', task_security_incident: '发现与任务相关的安全事件。', effects_verified: '已按要求核对独立效果证据。', effect_verified: '本项独立效果证据通过核验。' } as Record<string, string>)[reason] ?? `核验原因：${reason}`;
}
