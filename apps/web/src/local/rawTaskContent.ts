export type RawContentStatusState = 'disabled' | 'ready' | 'error';

export interface RawContentStatus {
  schema_version: 'local-raw-task-content-status/v1';
  status: RawContentStatusState;
  default_capture: false;
  retention_seconds: number | null;
  budget_bytes: number | null;
  activated_at: string | null;
}

export interface RawContentActivation {
  schema_version: 'local-raw-task-content-activation/v1';
  enabled: true;
  actor_ref: string;
  activated_at: string;
  retention_seconds: number;
  budget_bytes: number;
  key_fingerprint: string;
  signing_schema: 'local_canonical/v1';
  signature: string;
}

export interface RawContentPurgeResult {
  schema_version: 'local-raw-task-content-purge-result/v1';
  deleted_records: number;
  released_bytes: number;
}

const object = (value: unknown): value is Record<string, unknown> =>
  !!value && typeof value === 'object' && !Array.isArray(value);
const exact = (value: Record<string, unknown>, fields: readonly string[]) =>
  Object.keys(value).length === fields.length && fields.every((field) => field in value);
const integer = (value: unknown, minimum: number, maximum: number): value is number =>
  typeof value === 'number' && Number.isSafeInteger(value) && value >= minimum && value <= maximum;
const date = (value: unknown): value is string =>
  typeof value === 'string' && value.length <= 64 && Number.isFinite(Date.parse(value));
const digestRef = (value: unknown): value is string =>
  typeof value === 'string' && /^sha256:[a-f0-9]{64}$/.test(value);
const signature = (value: unknown): value is string =>
  typeof value === 'string' && /^[a-f0-9]{128}$/.test(value);

export function isRawContentStatus(value: unknown): value is RawContentStatus {
  if (!object(value) || !exact(value, ['schema_version', 'status', 'default_capture', 'retention_seconds', 'budget_bytes', 'activated_at']) ||
    value.schema_version !== 'local-raw-task-content-status/v1' || value.default_capture !== false ||
    !['disabled', 'ready', 'error'].includes(String(value.status))) return false;
  if (value.status === 'ready') {
    return integer(value.retention_seconds, 3600, 2592000) && integer(value.budget_bytes, 1048576, 1073741824) && date(value.activated_at);
  }
  return value.retention_seconds === null && value.budget_bytes === null && value.activated_at === null;
}

export function isRawContentActivation(value: unknown): value is RawContentActivation {
  return object(value) && exact(value, [
    'schema_version', 'enabled', 'actor_ref', 'activated_at', 'retention_seconds', 'budget_bytes',
    'key_fingerprint', 'signing_schema', 'signature',
  ]) && value.schema_version === 'local-raw-task-content-activation/v1' && value.enabled === true &&
    digestRef(value.actor_ref) && date(value.activated_at) && integer(value.retention_seconds, 3600, 2592000) &&
    integer(value.budget_bytes, 1048576, 1073741824) && digestRef(value.key_fingerprint) &&
    value.signing_schema === 'local_canonical/v1' && signature(value.signature);
}

export function isRawContentPurgeResult(value: unknown): value is RawContentPurgeResult {
  return object(value) && exact(value, ['schema_version', 'deleted_records', 'released_bytes']) &&
    value.schema_version === 'local-raw-task-content-purge-result/v1' && integer(value.deleted_records, 0, 4096) &&
    integer(value.released_bytes, 0, 1073741824);
}

export function formatBytes(value: number): string {
  if (value >= 1 << 30) return `${value / (1 << 30)} GiB`;
  if (value >= 1 << 20) return `${value / (1 << 20)} MiB`;
  if (value >= 1 << 10) return `${Math.round(value / (1 << 10))} KiB`;
  return `${value} B`;
}

export function formatRetention(seconds: number): string {
  if (seconds % 86400 === 0) return `${seconds / 86400} 天`;
  if (seconds % 3600 === 0) return `${seconds / 3600} 小时`;
  return `${seconds} 秒`;
}
