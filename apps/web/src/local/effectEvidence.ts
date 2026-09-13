export interface EffectEvidenceSummary {
  id: string; actionId: string; receiptId: string; effectType: string;
  sourceType: string; sourceId: string; independence: string; coverage: string;
  executionState: string; result: string; observedAt: string; digest: string;
  resourceRef: string; finding: string;
}
const object = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const text = (v: unknown): v is string => typeof v === 'string' && v.length > 0 && v.length <= 256;
const hex = (v: unknown, length: number) => typeof v === 'string' && new RegExp(`^[a-f0-9]{${length}}$`).test(v);
const member = (v: unknown, choices: string[]): v is string => typeof v === 'string' && choices.includes(v);
// Metadata projection only. The existing authenticated server verifies signatures.
export function readEffectEvidence(value: unknown, id: string, taskId: string): EffectEvidenceSummary | null {
  if (!object(value) || value.schema_version !== 'effect-evidence-record/v1' || value.task_id !== taskId ||
    value.signing_schema !== 'local_canonical/v1' || !hex(value.signature, 128) || !hex(value.request_digest, 64) ||
    !member(value.finding_code, ['', 'unauthorized_effect_observed', 'effect_scope_mismatch'])) return null;
  const e = value.evidence;
  if (!object(e) || e.schema_version !== 'effect-evidence/v1' || e.effect_evidence_id !== id || !text(e.action_id) ||
    !text(e.decision_receipt_id) || !text(e.effect_type) || !text(e.resource_ref) || !/^(filesystem|network|message):sha256:[a-f0-9]{64}$/.test(e.resource_ref) ||
    !hex(e.evidence_digest, 64) || !hex(e.signature, 128) || e.signing_schema !== 'local_canonical/v1' ||
    !text(e.observed_at) || !Number.isFinite(Date.parse(e.observed_at)) || !object(e.source)) return null;
  const source = e.source;
  if (!member(source.type, ['tool_report', 'host_observer', 'openshell', 'provider_audit', 'test_oracle', 'unknown']) || !text(source.source_id) ||
    !member(source.independence, ['self_reported', 'host_independent', 'external_independent', 'unknown']) ||
    !member(e.coverage, ['full', 'partial', 'unknown']) || !member(e.execution_state, ['requested', 'started', 'completed', 'failed', 'unknown']) ||
    !member(e.result, ['expected', 'unexpected', 'conflicting', 'unknown'])) return null;
  return { id, actionId: e.action_id, receiptId: e.decision_receipt_id, effectType: e.effect_type, sourceType: source.type, sourceId: source.source_id,
    independence: source.independence, coverage: e.coverage, executionState: e.execution_state, result: e.result, observedAt: e.observed_at,
    digest: e.evidence_digest as string, resourceRef: e.resource_ref, finding: value.finding_code };
}
