import type { TaskActivityItem } from './taskActivities';
import { isTaskExport } from './taskExport';

const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const exact = (v: Record<string, unknown>, names: string[]) => Object.keys(v).length === names.length && names.every((name) => Object.hasOwn(v, name));
const hash = (v: unknown): v is string => typeof v === 'string' && /^[a-f0-9]{64}$/.test(v);
const ref = (v: unknown): v is string => typeof v === 'string' && /^sha256:[a-f0-9]{64}$/.test(v);
const reason = (v: unknown) => typeof v === 'string' && /^[a-z][a-z0-9_]{0,127}$/.test(v);
const refs = (v: unknown, limit = 8192): v is string[] => Array.isArray(v) && v.length <= limit && v.every(ref) && new Set(v).size === v.length;
const status = (v: unknown) => ['verified', 'incomplete', 'conflicting', 'unknown', 'unavailable'].includes(String(v));

function validSource(row: unknown, receipt: Record<string, unknown>): boolean {
  if (!object(row) || !exact(row, ['seq', 'receipt_hash', 'status', 'source']) || row.seq !== receipt.seq || row.receipt_hash !== receipt.source_hash) return false;
  if (row.status === 'unavailable') return row.source === null;
  const source = row.source;
  if (row.status !== 'verified_source' || !object(source) || !exact(source, ['grant_ref', 'admission_ref', 'permission_digest', 'skill_name_ref', 'declared_version_ref', 'content_hash', 'import']) ||
    !ref(source.grant_ref) || !ref(source.admission_ref) || !hash(source.permission_digest) || !ref(source.skill_name_ref) ||
    (source.declared_version_ref !== null && !ref(source.declared_version_ref)) || !hash(source.content_hash)) return false;
  if (source.import === null) return true;
  return object(source.import) && exact(source.import, ['import_ref', 'artifact_digest', 'analysis_sha256']) &&
    ref(source.import.import_ref) && hash(source.import.artifact_digest) && hash(source.import.analysis_sha256);
}

function validCompletion(value: unknown): value is Record<string, unknown> {
  if (!object(value) || !exact(value, ['status', 'reason_code', 'requirements', 'incident_refs']) || !status(value.status) || !reason(value.reason_code) ||
    !Array.isArray(value.requirements) || value.requirements.length > 128 || !refs(value.incident_refs)) return false;
  if (value.status === 'unavailable' && (value.requirements.length !== 0 || value.incident_refs.length !== 0)) return false;
  const requirements = new Set<string>();
  for (const item of value.requirements) {
    if (!object(item) || !exact(item, ['requirement_ref', 'status', 'reason_code', 'evidence_refs']) || !ref(item.requirement_ref) || requirements.has(item.requirement_ref) ||
      !['verified', 'incomplete', 'conflicting', 'unknown'].includes(String(item.status)) || !reason(item.reason_code) || !refs(item.evidence_refs)) return false;
    requirements.add(item.requirement_ref as string);
    if (item.status === 'verified' && item.evidence_refs.length === 0) return false;
  }
  return value.status !== 'verified' || (value.requirements.length > 0 && value.incident_refs.length === 0 && value.requirements.every((item) => object(item) && item.status === 'verified'));
}

function validEffect(value: unknown): value is Record<string, unknown> {
  if (!object(value) || !exact(value, ['evidence_ref', 'action_ref', 'decision_receipt_ref', 'effect_type_ref', 'resource_ref', 'execution_state', 'source_type', 'source_ref', 'independence', 'coverage', 'result', 'evidence_digest', 'observed_at', 'finding_code'])) return false;
  return ref(value.evidence_ref) && ref(value.action_ref) && ref(value.decision_receipt_ref) && ref(value.effect_type_ref) &&
    typeof value.resource_ref === 'string' && /^(filesystem|network|message):sha256:[a-f0-9]{64}$/.test(value.resource_ref) &&
    ['requested', 'started', 'completed', 'failed', 'unknown'].includes(String(value.execution_state)) &&
    ['tool_report', 'host_observer', 'openshell', 'provider_audit', 'test_oracle', 'unknown'].includes(String(value.source_type)) && ref(value.source_ref) &&
    ['self_reported', 'host_independent', 'external_independent', 'unknown'].includes(String(value.independence)) &&
    ['full', 'partial', 'unknown'].includes(String(value.coverage)) && ['expected', 'unexpected', 'conflicting', 'unknown'].includes(String(value.result)) &&
    hash(value.evidence_digest) && typeof value.observed_at === 'string' && Number.isFinite(Date.parse(value.observed_at)) &&
    ['', 'unauthorized_effect_observed', 'effect_scope_mismatch'].includes(String(value.finding_code));
}

// The browser validates format and cross-field relationships. Cryptographic
// verification still requires an externally trusted public key outside the UI.
export function isTaskTraceExport(v: unknown, activity: TaskActivityItem, snapshot: string): v is Record<string, unknown> {
  const root = ['schema_version', 'attestation_scope', 'activity_id', 'snapshot', 'generated_at', 'source_count', 'source_tip_hash', 'source_last_seq', 'prefix_valid', 'history_integrity', 'incomplete', 'public_key_base64', 'receipts', 'sources', 'completion', 'effects', 'signing_schema', 'signature'];
  if (!object(v) || !exact(v, root) || v.schema_version !== 'local-task-trace-export/v1' || v.attestation_scope !== 'redacted_trace_projection_only' || typeof v.incomplete !== 'boolean' ||
    !Array.isArray(v.sources) || !Array.isArray(v.receipts) || v.sources.length !== v.receipts.length || !validCompletion(v.completion) || !Array.isArray(v.effects) || v.effects.length > 8192 || !v.effects.every(validEffect)) return false;
  const receipts = v.receipts;
  const sources = v.sources;
  const receiptProjection = { ...v, schema_version: 'local-task-activity-export/v1', attestation_scope: 'share_projection_only' } as Record<string, unknown>;
  for (const field of ['incomplete', 'sources', 'completion', 'effects']) delete receiptProjection[field];
  if (!isTaskExport(receiptProjection, activity, snapshot)) return false;
  if (!sources.every((row, index) => validSource(row, receipts[index] as Record<string, unknown>))) return false;
  const completion = v.completion as Record<string, unknown>;
  const incomplete = sources.some((row) => object(row) && row.status !== 'verified_source') || completion.status !== 'verified';
  if (v.incomplete !== incomplete) return false;
  const referenced = new Set<string>(completion.incident_refs as string[]);
  for (const item of completion.requirements as Record<string, unknown>[]) for (const evidence of item.evidence_refs as string[]) referenced.add(evidence);
  const effectRefs = (v.effects as Record<string, unknown>[]).map((effect) => effect.evidence_ref as string);
  return new Set(effectRefs).size === effectRefs.length && effectRefs.length === referenced.size && effectRefs.every((evidence) => referenced.has(evidence));
}
