import type { TaskActivityItem } from './taskActivities';
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const hex = (v: unknown, n = 64) => typeof v === 'string' && new RegExp(`^[a-f0-9]{${n}}$`).test(v);
const integer = (v: unknown): v is number => Number.isSafeInteger(v) && Number(v) >= 0;
const exact = (v: Record<string, unknown>, names: string[]) => Object.keys(v).length === names.length && names.every((n) => Object.hasOwn(v, n));
// Format and scope validation only. Preserve the signed document unchanged.
export function isTaskExport(v: unknown, activity: TaskActivityItem, snapshot: string): v is Record<string, unknown> {
  if (!object(v) || !exact(v, ['schema_version', 'attestation_scope', 'activity_id', 'snapshot', 'generated_at', 'source_count', 'source_tip_hash', 'source_last_seq', 'prefix_valid', 'history_integrity', 'public_key_base64', 'receipts', 'signing_schema', 'signature']) ||
    activity.attribution !== 'bound' || v.schema_version !== 'local-task-activity-export/v1' || v.attestation_scope !== 'share_projection_only' ||
    v.activity_id !== activity.activity_id || !hex(v.activity_id) || v.snapshot !== snapshot || !hex(v.snapshot) ||
    typeof v.generated_at !== 'string' || !Number.isFinite(Date.parse(v.generated_at)) || v.prefix_valid !== true ||
    !['verified', 'unknown', 'failed'].includes(String(v.history_integrity)) || !integer(v.source_count) || v.source_count < activity.receipt_count || v.source_count > 100000 ||
    !integer(v.source_last_seq) || v.source_last_seq < activity.last_seq || !hex(v.source_tip_hash) ||
    typeof v.public_key_base64 !== 'string' || !/^[A-Za-z0-9+/]{43}=$/.test(v.public_key_base64) || v.signing_schema !== 'local_canonical/v1' || !hex(v.signature, 128) ||
    !Array.isArray(v.receipts) || v.receipts.length !== activity.receipt_count || v.receipts.length < 1 || v.receipts.length > 10000) return false;
  let previous = -1;
  for (const row of v.receipts) {
    if (!object(row) || !exact(row, ['seq', 'issued_at', 'receipt_ref', 'tool_ref', 'action', 'source_hash']) || !integer(row.seq) || row.seq <= previous || row.seq < activity.first_seq || row.seq > activity.last_seq ||
      typeof row.issued_at !== 'string' || row.issued_at.length > 40 || (row.issued_at !== '' && !Number.isFinite(Date.parse(row.issued_at))) ||
      !['allow', 'deny', 'hold', 'redact', 'unknown'].includes(String(row.action)) || !hex(row.source_hash) ||
      ![row.receipt_ref, row.tool_ref].every((ref) => typeof ref === 'string' && /^sha256:[a-f0-9]{64}$/.test(ref))) return false;
    previous = row.seq;
  }
  return v.receipts[0].seq === activity.first_seq && previous === activity.last_seq;
}
