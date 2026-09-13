import type { TaskActivityDetail } from './taskActivities';
export interface SkillSource {
  grant_id: string; admission_id: string; permission_digest: string; skill_name: string;
  declared_version: string | null; content_hash: string;
  import: null | { schema_version: string; import_id: string; artifact_digest: string; analysis_sha256: string };
}
export interface ActivitySourceRow { seq: number; receipt_hash: string; status: 'verified_source' | 'unavailable' | 'unattributed'; source: SkillSource | null }
export interface ActivitySources { schema_version: string; activity_id: string; snapshot: string; view: string; offset: number; total: number; next_offset: number | null; items: ActivitySourceRow[] }
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const exact = (v: Record<string, unknown>, keys: string[]) => Object.keys(v).length === keys.length && keys.every((k) => Object.hasOwn(v, k));
const hash = (v: unknown) => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const label = (v: unknown): v is string => typeof v === 'string' && v.length > 0 && [...v].length <= 256 && !/[\u0000-\u001f\u007f-\u009f]/.test(v);
export function isActivitySources(v: unknown, detail: TaskActivityDetail): v is ActivitySources {
  if (!object(v) || !exact(v, ['schema_version', 'activity_id', 'snapshot', 'view', 'offset', 'total', 'next_offset', 'items']) ||
    v.schema_version !== 'local-task-activity-sources/v1' || v.activity_id !== detail.activity.activity_id || v.snapshot !== detail.snapshot ||
    v.view !== detail.view || v.offset !== detail.offset || v.total !== detail.total || v.next_offset !== detail.next_offset || !Array.isArray(v.items) || v.items.length !== detail.receipts.length) return false;
  return v.items.every((row, i) => {
    const receipt = detail.receipts[i];
    if (!object(row) || !exact(row, ['seq', 'receipt_hash', 'status', 'source']) || row.seq !== receipt.seq || row.receipt_hash !== receipt.hash) return false;
    if (detail.view === 'unassigned') return row.status === 'unattributed' && row.source === null;
    if (row.status === 'unavailable') return row.source === null;
    const s = row.source;
    if (row.status !== 'verified_source' || !object(s) || !exact(s, ['grant_id', 'admission_id', 'permission_digest', 'skill_name', 'declared_version', 'content_hash', 'import']) ||
      !label(s.grant_id) || s.grant_id !== receipt.matched_grant_id || !label(s.admission_id) || !hash(s.permission_digest) || !label(s.skill_name) ||
      (s.declared_version !== null && !label(s.declared_version)) || !hash(s.content_hash)) return false;
    if (s.import === null) return !s.admission_id.startsWith('adm-si-');
    const imp = s.import;
    return s.admission_id.startsWith('adm-si-') && object(imp) && exact(imp, ['schema_version', 'import_id', 'artifact_digest', 'analysis_sha256']) &&
      imp.schema_version === 'local-skill-import-permission-source/v1' && typeof imp.import_id === 'string' && /^si-[a-f0-9]{32}$/.test(imp.import_id) && hash(imp.artifact_digest) && hash(imp.analysis_sha256);
  });
}
