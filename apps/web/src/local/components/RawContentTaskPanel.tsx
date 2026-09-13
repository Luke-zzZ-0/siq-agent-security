import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import { LocalApiError, localApi } from '../api';
import { formatBytes, formatRetention, type RawContentActivation, type RawContentStatus } from '../rawTaskContent';
import type {
  RawContentGrantView,
  RawContentKind,
  RawContentRecord,
  RawContentRecordContent,
} from '../rawTaskContentManagement';

interface Props {
  taskId: string;
  actorId: string;
}

interface LoadedState {
  loading: boolean;
  status?: RawContentStatus;
  activation?: RawContentActivation;
  grants?: RawContentGrantView[];
  records?: RawContentRecord[];
  error?: string;
}

const kindLabels: Record<RawContentKind, string> = {
  input: '输入', parameters: '参数', output: '结果', note: '备注',
};
const statusLabels = { active: '采集授权有效', expired: '授权已到期', revoked: '授权已撤销' } as const;
const recordStatusLabels = { active: '可查看', expired: '已到期' } as const;

const operationError = (error: unknown) => {
  if (error instanceof LocalApiError) {
    if (error.status === 404) return '目标记录或授权已不存在，请刷新后重试。';
    if (error.status === 409) return '原文授权状态已变化，请刷新后重新确认。';
    if (error.status === 410) return '原文已到期，不能再查看；仍可明确删除密文。';
    if (error.status === 502) return '原文服务响应与当前任务或选择不匹配，已停止本次操作。';
    if (error.status === 503) return '原文仓完整性当前无法验证，已停止本次操作。';
  }
  return error instanceof Error ? error.message : '原文操作失败，请刷新后重试。';
};

export default function RawContentTaskPanel({ taskId, actorId }: Props) {
  const [retry, setRetry] = useState(0);
  const [state, setState] = useState<LoadedState>({ loading: true });
  const [selectedKinds, setSelectedKinds] = useState<RawContentKind[]>([]);
  const [duration, setDuration] = useState(3600);
  const [retention, setRetention] = useState(3600);
  const [maxBytes, setMaxBytes] = useState(65536);
  const [confirmGrant, setConfirmGrant] = useState(false);
  const [revokeTarget, setRevokeTarget] = useState<RawContentGrantView | null>(null);
  const [confirmRevoke, setConfirmRevoke] = useState(false);
  const [readTarget, setReadTarget] = useState<RawContentRecord | null>(null);
  const [confirmRead, setConfirmRead] = useState(false);
  const [content, setContent] = useState<RawContentRecordContent | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<RawContentRecord | null>(null);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(null);

  const clearSensitive = () => {
    setReadTarget(null);
    setConfirmRead(false);
    setContent(null);
  };
  const refresh = () => {
    clearSensitive();
    setDeleteTarget(null);
    setConfirmDelete(false);
    setRevokeTarget(null);
    setConfirmRevoke(false);
    setNotice(null);
    setRetry((value) => value + 1);
  };

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setState({ loading: true });
    setContent(null);
    localApi.rawContentStatus(controller.signal).then(async (status) => {
      if (status.status !== 'ready') {
        if (active) setState({ loading: false, status });
        return;
      }
      const [activation, grants, records] = await Promise.all([
        localApi.rawContentActivation(controller.signal),
        localApi.rawContentGrants(taskId, controller.signal),
        localApi.rawContentRecords(taskId, controller.signal),
      ]);
      if (active) {
        setRetention((value) => Math.min(value, activation.retention_seconds));
        setState({ loading: false, status, activation, grants, records });
      }
    }).catch((error: unknown) => {
      if (active && !controller.signal.aborted) setState({ loading: false, error: operationError(error) });
    });
    return () => {
      active = false;
      controller.abort();
    };
  }, [taskId, retry]);

  const retentionOptions = useMemo(() => {
    const maximum = state.activation?.retention_seconds ?? 3600;
    return Array.from(new Set([3600, 86400, 604800, 2592000, maximum]))
      .filter((value) => value <= maximum)
      .sort((left, right) => left - right);
  }, [state.activation?.retention_seconds]);

  const toggleKind = (kind: RawContentKind) => setSelectedKinds((current) =>
    current.includes(kind) ? current.filter((item) => item !== kind) : [...current, kind].sort(
      (left, right) => Object.keys(kindLabels).indexOf(left) - Object.keys(kindLabels).indexOf(right),
    ));

  const createGrant = (event: FormEvent) => {
    event.preventDefault();
    if (!confirmGrant || !actorId.trim() || selectedKinds.length === 0 || busy) return;
    clearSensitive();
    setRevokeTarget(null);
    setConfirmRevoke(false);
    setDeleteTarget(null);
    setConfirmDelete(false);
    setBusy('grant');
    setNotice(null);
    localApi.createRawContentGrant({
      taskId, kinds: selectedKinds, actorId: actorId.trim(), durationSeconds: duration,
      retentionSeconds: retention, maxPlaintextBytes: maxBytes,
    }).then(() => {
      setSelectedKinds([]);
      setConfirmGrant(false);
      setNotice({ text: '已为此任务创建原文采集授权。实际写入仍需同一运行时身份与会话通过复验。', error: false });
      setRetry((value) => value + 1);
    }).catch((error: unknown) => setNotice({ text: operationError(error), error: true }))
      .finally(() => setBusy(null));
  };

  const revoke = () => {
    if (!revokeTarget || !confirmRevoke || !actorId.trim() || busy) return;
    setBusy('revoke');
    setNotice(null);
    localApi.revokeRawContentGrant(revokeTarget, actorId.trim()).then(() => {
      setRevokeTarget(null);
      setConfirmRevoke(false);
      setNotice({ text: '原文采集授权已终止；既有密文保持原保留期，可另行查看或删除。', error: false });
      setRetry((value) => value + 1);
    }).catch((error: unknown) => setNotice({ text: operationError(error), error: true }))
      .finally(() => setBusy(null));
  };

  const read = () => {
    if (!readTarget || !confirmRead || busy) return;
    setBusy('read');
    setNotice(null);
    setContent(null);
    localApi.readRawContentRecord(taskId, readTarget).then((result) => {
      setContent(result);
      setConfirmRead(false);
    }).catch((error: unknown) => {
      setContent(null);
      setNotice({ text: operationError(error), error: true });
    }).finally(() => setBusy(null));
  };

  const remove = () => {
    if (!deleteTarget || !confirmDelete || busy) return;
    setBusy('delete');
    setNotice(null);
    localApi.deleteRawContentRecord(taskId, deleteTarget).then(() => {
      if (readTarget?.record_id === deleteTarget.record_id) clearSensitive();
      setDeleteTarget(null);
      setConfirmDelete(false);
      setNotice({ text: '所选原文密文已删除；任务、授权、回执和结果证据未修改。', error: false });
      setRetry((value) => value + 1);
    }).catch((error: unknown) => setNotice({ text: operationError(error), error: true }))
      .finally(() => setBusy(null));
  };

  const status = state.status;
  return <section className="raw-content-task-panel" aria-label="任务原文管理" aria-busy={state.loading || busy !== null}>
    <div className="section-heading-row">
      <div>
        <h3>任务原文（按需）</h3>
        <p className="page-desc">原文是可删除的辅助内容，不属于默认脱敏回执或完整追溯包。secret 与凭据始终排除。</p>
      </div>
      <button type="button" className="btn btn-sm" disabled={state.loading || busy !== null} onClick={refresh}>刷新原文状态</button>
    </div>
    {state.loading ? <p role="status">正在核对原文仓、任务授权和密文元数据…</p> : null}
    {state.error ? <p role="alert" className="action-error">{state.error}</p> : null}
    {status?.status === 'disabled' ? <p role="status">原文仓当前关闭。可前往 <Link to="/settings">设置</Link> 查看隐私边界并显式启用。</p> : null}
    {status?.status === 'error' ? <p role="alert" className="action-error">原文仓完整性异常；此任务的授权和记录操作已关闭。</p> : null}
    {status?.status === 'ready' && state.activation ? <>
      <p role="status"><strong>仓已启用，当前任务仍按下列独立授权执行。</strong>仓级启用不表示此任务正在采集。</p>
      <form className="raw-content-grant-form" onSubmit={createGrant}>
        <fieldset disabled={busy !== null}>
          <legend>创建此任务的采集授权</legend>
          <div className="raw-content-kind-options">
            {(Object.keys(kindLabels) as RawContentKind[]).map((kind) => <label key={kind}>
              <input type="checkbox" checked={selectedKinds.includes(kind)} onChange={() => toggleKind(kind)} />
              {kindLabels[kind]}
            </label>)}
          </div>
          <div className="form-row">
            <div className="field field-flush"><label htmlFor={`raw-duration-${taskId}`}>授权有效期</label><select id={`raw-duration-${taskId}`} value={duration} onChange={(event) => setDuration(Number(event.target.value))}>
              <option value={600}>10 分钟</option><option value={3600}>1 小时</option><option value={28800}>8 小时</option><option value={86400}>24 小时</option>
            </select></div>
            <div className="field field-flush"><label htmlFor={`raw-record-retention-${taskId}`}>记录保留期</label><select id={`raw-record-retention-${taskId}`} value={retention} onChange={(event) => setRetention(Number(event.target.value))}>
              {retentionOptions.map((value) => <option value={value} key={value}>{formatRetention(value)}</option>)}
            </select></div>
            <div className="field field-flush"><label htmlFor={`raw-max-${taskId}`}>单条原文上限</label><select id={`raw-max-${taskId}`} value={maxBytes} onChange={(event) => setMaxBytes(Number(event.target.value))}>
              <option value={4096}>4 KiB</option><option value={65536}>64 KiB</option><option value={1048576}>1 MiB</option>
            </select></div>
          </div>
          <p>操作者：<strong>{actorId.trim() || '请先在设置页填写人工身份'}</strong></p>
          <label className="confirmation-check"><input type="checkbox" checked={confirmGrant} onChange={(event) => setConfirmGrant(event.target.checked)} />我确认仅为当前任务授权所选内容种类；实际采集仍需匹配的智能体、会话与运行时权限。</label>
          <button type="submit" className="btn btn-primary" disabled={busy !== null || !confirmGrant || !actorId.trim() || selectedKinds.length === 0}>{busy === 'grant' ? '正在创建…' : '创建任务原文授权'}</button>
        </fieldset>
      </form>

      <div className="raw-content-subsection">
        <h4>此任务的采集授权</h4>
        {state.grants?.length ? <div className="raw-content-items">{state.grants.map((view) => <article className="raw-content-item" key={view.grant.grant_id}>
          <div><strong>{statusLabels[view.status]}</strong><span className="mono">{view.grant.grant_id}</span></div>
          <p>{view.grant.kinds.map((kind) => kindLabels[kind]).join('、')} · 单条 {formatBytes(view.grant.max_plaintext_bytes)} · 保留 {formatRetention(view.grant.retention_seconds)}</p>
          <p className="page-desc">{new Date(view.grant.issued_at).toLocaleString()} 至 {new Date(view.grant.expires_at).toLocaleString()}</p>
          {view.status === 'active' ? <button type="button" className="btn btn-sm" disabled={busy !== null} onClick={() => { clearSensitive(); setDeleteTarget(null); setConfirmDelete(false); setRevokeTarget(view); setConfirmRevoke(false); }}>撤销这份采集授权</button> : null}
        </article>)}</div> : <p>此任务尚无原文采集授权。</p>}
        {revokeTarget ? <div className="raw-content-action" role="region" aria-label="撤销原文授权">
          <p>将终止 <span className="mono">{revokeTarget.grant.grant_id}</span>。这不会删除既有密文。</p>
          <label className="confirmation-check"><input type="checkbox" checked={confirmRevoke} onChange={(event) => setConfirmRevoke(event.target.checked)} />确认撤销此任务的这份原文采集授权。</label>
          <div className="row-actions"><button type="button" className="btn btn-danger" disabled={busy !== null || !confirmRevoke || !actorId.trim()} onClick={revoke}>{busy === 'revoke' ? '正在撤销…' : '确认撤销'}</button><button type="button" className="btn" disabled={busy !== null} onClick={() => { setRevokeTarget(null); setConfirmRevoke(false); }}>取消</button></div>
        </div> : null}
      </div>

      <div className="raw-content-subsection">
        <h4>此任务的原文记录</h4>
        {state.records?.length ? <div className="raw-content-items">{state.records.map((record) => <article className="raw-content-item" key={record.record_id}>
          <div><strong>{kindLabels[record.kind]} · {recordStatusLabels[record.status]}</strong><span className="mono">{record.record_id}</span></div>
          <p>{formatBytes(record.plaintext_bytes)} · 已排除 {record.omitted_secret_count} 个 secret 字段 · 到期 {new Date(record.expires_at).toLocaleString()}</p>
          <div className="row-actions">{record.status === 'active' ? <button type="button" className="btn btn-sm" disabled={busy !== null} onClick={() => { clearSensitive(); setRevokeTarget(null); setConfirmRevoke(false); setDeleteTarget(null); setConfirmDelete(false); setReadTarget(record); }}>二次确认后查看</button> : null}<button type="button" className="btn btn-sm" disabled={busy !== null} onClick={() => { clearSensitive(); setRevokeTarget(null); setConfirmRevoke(false); setDeleteTarget(record); setConfirmDelete(false); }}>删除这条密文</button></div>
        </article>)}</div> : <p>此任务当前没有原文记录。授权本身不会创建记录。</p>}
        {readTarget ? <div className="raw-content-action" role="region" aria-label="查看任务原文">
          <div className="section-heading-row"><p>记录 <span className="mono">{readTarget.record_id}</span>。读取后，辅助原文会显示在当前页面。</p><button type="button" className="btn btn-sm" disabled={busy !== null} onClick={clearSensitive}>关闭并清除</button></div>
          {!content ? <><label className="confirmation-check"><input type="checkbox" checked={confirmRead} onChange={(event) => setConfirmRead(event.target.checked)} />我确认现在把这条记录的已过滤原文显示在当前页面；离开或刷新后清除。</label><button type="button" className="btn" disabled={busy !== null || !confirmRead} onClick={read}>{busy === 'read' ? '正在读取…' : '读取一次'}</button></> : <div className="raw-content-plaintext" role="region" aria-label="已解密辅助原文">
            <p><strong>当前页面中的辅助原文</strong> · {formatBytes(content.record.plaintext_bytes)}</p>
            {content.fields.map((field, index) => <div key={`${index}:${field.path}`}><code>{field.path}</code><pre>{field.value}</pre></div>)}
          </div>}
        </div> : null}
        {deleteTarget ? <div className="raw-content-action" role="region" aria-label="删除任务原文">
          <p>将永久删除密文 <span className="mono">{deleteTarget.record_id}</span>。任务、回执和证据保持不变。</p>
          <label className="confirmation-check"><input type="checkbox" checked={confirmDelete} onChange={(event) => setConfirmDelete(event.target.checked)} />确认删除完整 ID 为 {deleteTarget.record_id} 的原文密文。</label>
          <div className="row-actions"><button type="button" className="btn btn-danger" disabled={busy !== null || !confirmDelete} onClick={remove}>{busy === 'delete' ? '正在删除…' : '确认删除密文'}</button><button type="button" className="btn" disabled={busy !== null} onClick={() => { setDeleteTarget(null); setConfirmDelete(false); }}>取消</button></div>
        </div> : null}
      </div>
    </> : null}
    {notice ? <p className={notice.error ? 'action-error' : 'action-success'} role={notice.error ? 'alert' : 'status'}>{notice.text}</p> : null}
  </section>;
}
