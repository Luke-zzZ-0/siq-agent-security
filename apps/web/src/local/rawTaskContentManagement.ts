export type RawContentKind = 'input' | 'parameters' | 'output' | 'note';
export type RawContentGrantStatus = 'active' | 'expired' | 'revoked';

export interface RawContentGrantRequest {
  taskId: string;
  kinds: RawContentKind[];
  actorId: string;
  durationSeconds: number;
  retentionSeconds: number;
  maxPlaintextBytes: number;
}

export interface RawContentGrant {
  schema_version: 'local-raw-task-content-grant/v1';
  grant_id: string;
  task_ref: string;
  kinds: RawContentKind[];
  actor_ref: string;
  issued_at: string;
  expires_at: string;
  retention_seconds: number;
  max_plaintext_bytes: number;
  signing_schema: 'local_canonical/v1';
  signature: string;
}

export interface RawContentRevocation {
  schema_version: 'local-raw-task-content-revocation/v1';
  grant_id: string;
  expected_grant_signature: string;
  actor_ref: string;
  revoked_at: string;
  reason_code: 'raw_content_capture_revoked';
  signing_schema: 'local_canonical/v1';
  signature: string;
}

export interface RawContentGrantView {
  schema_version: 'local-raw-task-content-grant-view/v1';
  status: RawContentGrantStatus;
  grant: RawContentGrant;
  revocation: RawContentRevocation | null;
}

export interface RawContentRecord {
  schema_version: 'local-raw-task-content-record/v1';
  status: 'active' | 'expired';
  record_id: string;
  task_ref: string;
  kind: RawContentKind;
  created_at: string;
  expires_at: string;
  plaintext_sha256: string;
  plaintext_bytes: number;
  omitted_secret_count: number;
}

export interface RawContentRecordContent {
  schema_version: 'local-raw-task-content-record-content/v1';
  contains_plaintext: true;
  record: RawContentRecord;
  fields: { path: string; value: string }[];
}

const kinds = ['input', 'parameters', 'output', 'note'] as const;
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
const grantId = (value: unknown): value is string =>
  typeof value === 'string' && /^rawgrant-[a-f0-9]{32}$/.test(value);
const recordId = (value: unknown): value is string =>
  typeof value === 'string' && /^raw-[a-f0-9]{32}$/.test(value);
const validKinds = (value: unknown): value is RawContentKind[] =>
  Array.isArray(value) && value.length >= 1 && value.length <= kinds.length &&
  new Set(value).size === value.length && value.every((kind) => kinds.includes(kind));
const sameKinds = (left: RawContentKind[], right: RawContentKind[]) =>
  left.length === right.length && left.every((kind, index) => kind === right[index]);

export async function rawContentTaskRef(taskId: string): Promise<string> {
  const digest = await globalThis.crypto.subtle.digest('SHA-256', new TextEncoder().encode(taskId));
  return `sha256:${Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, '0')).join('')}`;
}

export function isRawContentGrant(
  value: unknown,
  expectedTaskRef?: string,
  expectedRequest?: RawContentGrantRequest,
): value is RawContentGrant {
  if (!object(value) || !exact(value, [
    'schema_version', 'grant_id', 'task_ref', 'kinds', 'actor_ref', 'issued_at', 'expires_at',
    'retention_seconds', 'max_plaintext_bytes', 'signing_schema', 'signature',
  ]) || value.schema_version !== 'local-raw-task-content-grant/v1' || !grantId(value.grant_id) ||
    !digestRef(value.task_ref) || !validKinds(value.kinds) || !digestRef(value.actor_ref) ||
    !date(value.issued_at) || !date(value.expires_at) || Date.parse(value.expires_at) <= Date.parse(value.issued_at) ||
    !integer(value.retention_seconds, 3600, 2592000) || !integer(value.max_plaintext_bytes, 1, 1048576) ||
    value.signing_schema !== 'local_canonical/v1' || !signature(value.signature)) return false;
  if (expectedTaskRef && value.task_ref !== expectedTaskRef) return false;
  return !expectedRequest || (sameKinds(value.kinds, expectedRequest.kinds) &&
    value.retention_seconds === expectedRequest.retentionSeconds &&
    value.max_plaintext_bytes === expectedRequest.maxPlaintextBytes);
}

export function isRawContentRevocation(value: unknown, grant: RawContentGrant): value is RawContentRevocation {
  return object(value) && exact(value, [
    'schema_version', 'grant_id', 'expected_grant_signature', 'actor_ref', 'revoked_at',
    'reason_code', 'signing_schema', 'signature',
  ]) && value.schema_version === 'local-raw-task-content-revocation/v1' &&
    value.grant_id === grant.grant_id && value.expected_grant_signature === grant.signature &&
    digestRef(value.actor_ref) && date(value.revoked_at) &&
    value.reason_code === 'raw_content_capture_revoked' && value.signing_schema === 'local_canonical/v1' &&
    signature(value.signature);
}

export function isRawContentGrantView(value: unknown): value is RawContentGrantView {
  if (!object(value) || !exact(value, ['schema_version', 'status', 'grant', 'revocation']) ||
    value.schema_version !== 'local-raw-task-content-grant-view/v1' ||
    !['active', 'expired', 'revoked'].includes(String(value.status)) || !isRawContentGrant(value.grant)) return false;
  return value.status === 'revoked'
    ? isRawContentRevocation(value.revocation, value.grant)
    : value.revocation === null;
}

export function readRawContentGrants(value: unknown): RawContentGrantView[] | null {
  if (!object(value) || !exact(value, ['schema_version', 'items']) ||
    value.schema_version !== 'local-raw-task-content-grants/v1' || !Array.isArray(value.items) ||
    value.items.length > 4096 || !value.items.every(isRawContentGrantView)) return null;
  const items = value.items as RawContentGrantView[];
  for (let index = 1; index < items.length; index += 1) {
    if (items[index - 1].grant.grant_id >= items[index].grant.grant_id) return null;
  }
  return items;
}

export function isRawContentRecord(value: unknown, expectedTaskRef?: string): value is RawContentRecord {
  return object(value) && exact(value, [
    'schema_version', 'status', 'record_id', 'task_ref', 'kind', 'created_at', 'expires_at',
    'plaintext_sha256', 'plaintext_bytes', 'omitted_secret_count',
  ]) && value.schema_version === 'local-raw-task-content-record/v1' &&
    ['active', 'expired'].includes(String(value.status)) && recordId(value.record_id) &&
    digestRef(value.task_ref) && (!expectedTaskRef || value.task_ref === expectedTaskRef) &&
    kinds.includes(value.kind as RawContentKind) && date(value.created_at) && date(value.expires_at) &&
    Date.parse(value.expires_at) > Date.parse(value.created_at) &&
    typeof value.plaintext_sha256 === 'string' && /^[a-f0-9]{64}$/.test(value.plaintext_sha256) &&
    integer(value.plaintext_bytes, 1, 1048576) && integer(value.omitted_secret_count, 0, 1024);
}

export function readRawContentRecords(value: unknown, expectedTaskRef: string): RawContentRecord[] | null {
  if (!object(value) || !exact(value, ['schema_version', 'items']) ||
    value.schema_version !== 'local-raw-task-content-records/v1' || !Array.isArray(value.items) ||
    value.items.length > 4096 || !value.items.every((item) => isRawContentRecord(item, expectedTaskRef))) return null;
  const items = value.items as RawContentRecord[];
  for (let index = 1; index < items.length; index += 1) {
    if (items[index - 1].record_id >= items[index].record_id) return null;
  }
  return items;
}

const sameRecord = (left: RawContentRecord, right: RawContentRecord) =>
  Object.keys(left).every((key) => left[key as keyof RawContentRecord] === right[key as keyof RawContentRecord]);

export function isRawContentRecordContent(
  value: unknown,
  expected: RawContentRecord,
): value is RawContentRecordContent {
  if (!object(value) || !exact(value, ['schema_version', 'contains_plaintext', 'record', 'fields']) ||
    value.schema_version !== 'local-raw-task-content-record-content/v1' || value.contains_plaintext !== true ||
    !isRawContentRecord(value.record, expected.task_ref) || value.record.status !== 'active' ||
    !sameRecord(value.record, expected) || !Array.isArray(value.fields) ||
    value.fields.length < 1 || value.fields.length > 1024) return false;
  return value.fields.every((field) => object(field) && exact(field, ['path', 'value']) &&
    typeof field.path === 'string' && field.path.length >= 1 && field.path.length <= 256 &&
    typeof field.value === 'string' && field.value.length >= 1 &&
    new TextEncoder().encode(field.value).length <= 1048576);
}

export function isRawContentRecordDeleted(value: unknown, expectedRecordId: string): boolean {
  return object(value) && exact(value, ['schema_version', 'record_id', 'deleted']) &&
    value.schema_version === 'local-raw-task-content-record-deleted/v1' &&
    value.record_id === expectedRecordId && recordId(value.record_id) && value.deleted === true;
}
