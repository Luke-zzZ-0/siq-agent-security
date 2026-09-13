import { useEffect, useState, type FormEvent } from 'react';
import { LocalApiError, localApi } from '../api';
import { formatBytes, formatRetention, type RawContentActivation, type RawContentStatus } from '../rawTaskContent';

interface Props {
  actorId: string;
}

const errorText = (error: unknown) => error instanceof LocalApiError && error.status === 409
  ? '原文仓已经使用另一组固定限制启用，不能直接覆盖。请刷新状态。'
  : error instanceof LocalApiError && error.status === 503
    ? '原文仓状态或密钥校验失败。请运行服务状态检查，系统不会自动覆盖。'
    : error instanceof LocalApiError && error.status === 502 && error.message === 'raw_task_content_status_invalid'
      ? '原文记录状态响应不完整。系统不会把它视为已启用。'
      : error instanceof LocalApiError && error.status === 502 && error.message === 'raw_task_content_activation_invalid'
        ? '原文仓启用记录响应不完整。系统不会展示或覆盖现有设置。'
        : error instanceof LocalApiError && error.status === 502 && error.message === 'raw_task_content_purge_invalid'
          ? '原文清理结果响应不完整，请刷新状态后重试。'
    : error instanceof Error ? error.message : '原文仓操作失败，请重试。';

export default function RawContentPrivacyPanel({ actorId }: Props) {
  const [retry, setRetry] = useState(0);
  const [state, setState] = useState<{ loading: boolean; status?: RawContentStatus; activation?: RawContentActivation; error?: string }>({ loading: true });
  const [retention, setRetention] = useState(86400);
  const [budget, setBudget] = useState(64 << 20);
  const [confirmEnable, setConfirmEnable] = useState(false);
  const [confirmPurge, setConfirmPurge] = useState(false);
  const [busy, setBusy] = useState<'enable' | 'purge' | null>(null);
  const [notice, setNotice] = useState<{ text: string; error: boolean } | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    setState({ loading: true });
    localApi.rawContentStatus(controller.signal).then(async (status) => {
      const activation = status.status === 'ready' ? await localApi.rawContentActivation(controller.signal) : undefined;
      if (active) setState({ loading: false, status, activation });
    }).catch((error: unknown) => {
      if (active) setState({ loading: false, error: errorText(error) });
    });
    return () => { active = false; controller.abort(); };
  }, [retry]);

  const refresh = () => {
    setNotice(null);
    setRetry((value) => value + 1);
  };
  const enable = (event: FormEvent) => {
    event.preventDefault();
    if (!confirmEnable || !actorId.trim()) return;
    setBusy('enable');
    setNotice(null);
    localApi.activateRawContent(actorId.trim(), retention, budget).then(() => {
      setConfirmEnable(false);
      setNotice({ text: '原文仓已启用。默认仍不采集，需在具体任务中单独授权。', error: false });
      setRetry((value) => value + 1);
      window.dispatchEvent(new Event('siq:raw-content-status-changed'));
    }).catch((error: unknown) => setNotice({ text: errorText(error), error: true })).finally(() => setBusy(null));
  };
  const purge = () => {
    if (!confirmPurge) return;
    setBusy('purge');
    setNotice(null);
    localApi.purgeExpiredRawContent().then((result) => {
      setConfirmPurge(false);
      setNotice({ text: `已清理 ${result.deleted_records} 条到期密文，释放 ${formatBytes(result.released_bytes)}。任务回执和追溯记录未改动。`, error: false });
      window.dispatchEvent(new Event('siq:raw-content-status-changed'));
    }).catch((error: unknown) => setNotice({ text: errorText(error), error: true })).finally(() => setBusy(null));
  };

  const status = state.status;
  return <div className="card raw-content-privacy" aria-busy={state.loading || busy !== null}>
    <div className="section-heading-row">
      <div>
        <h2>按需原文与本机存储</h2>
        <p className="page-desc">默认不保存参数或结果原文。启用后也只允许具体任务、内容种类和时间范围内的采集，secret 与凭据始终排除。</p>
      </div>
      <button type="button" className="btn btn-sm" disabled={state.loading || busy !== null} onClick={refresh}>刷新状态</button>
    </div>
    {state.loading ? <p role="status">正在核对签名启用记录与独立密钥…</p> : null}
    {state.error ? <p role="alert" className="action-error">{state.error}</p> : null}
    {status?.status === 'disabled' ? <form onSubmit={enable}>
      <p role="status"><strong>原文记录关闭。</strong>当前只有脱敏回执和证据摘要，原文仓尚未创建有效启用记录。</p>
      <div className="form-row">
        <div className="field field-flush">
          <label htmlFor="raw-retention">原文保留期</label>
          <select id="raw-retention" value={retention} onChange={(event) => setRetention(Number(event.target.value))}>
            <option value={3600}>1 小时</option>
            <option value={86400}>1 天</option>
            <option value={604800}>7 天</option>
            <option value={2592000}>30 天</option>
          </select>
        </div>
        <div className="field field-flush">
          <label htmlFor="raw-budget">最大磁盘占用</label>
          <select id="raw-budget" value={budget} onChange={(event) => setBudget(Number(event.target.value))}>
            <option value={16 << 20}>16 MiB</option>
            <option value={64 << 20}>64 MiB</option>
            <option value={256 << 20}>256 MiB</option>
            <option value={1 << 30}>1 GiB</option>
          </select>
        </div>
      </div>
      <p>操作者：<strong>{actorId.trim() || '请先在“人工身份”中填写'}</strong>。首版启用限制不可原地改写。</p>
      <label className="confirmation-check"><input type="checkbox" checked={confirmEnable} onChange={(event) => setConfirmEnable(event.target.checked)} />我理解原文是独立的可删除辅助内容，不进入默认脱敏追溯包；每个任务仍需单独授权。</label>
      <button type="submit" className="btn btn-primary" disabled={busy !== null || !confirmEnable || !actorId.trim()}>{busy === 'enable' ? '正在启用…' : '启用按需原文仓'}</button>
    </form> : null}
    {status?.status === 'ready' && state.activation ? <>
      <p role="status"><strong>原文仓已启用，默认采集仍为关闭。</strong>只有获得单独授权的任务才能写入。</p>
      <dl className="summary-grid">
        <div><dt>启用时间</dt><dd>{new Date(state.activation.activated_at).toLocaleString()}</dd></div>
        <div><dt>保留期</dt><dd>{formatRetention(state.activation.retention_seconds)}</dd></div>
        <div><dt>磁盘上限</dt><dd>{formatBytes(state.activation.budget_bytes)}</dd></div>
        <div><dt>签名状态</dt><dd>服务端已复验</dd></div>
      </dl>
      <p className="page-desc">当前限制来自不可变签名启用记录。任务授权和原文查看在对应任务活动详情中完成。</p>
      <label className="confirmation-check"><input type="checkbox" checked={confirmPurge} onChange={(event) => setConfirmPurge(event.target.checked)} />仅删除已达到保留期限的独立密文，不修改任务、回执或证据。</label>
      <button type="button" className="btn" disabled={busy !== null || !confirmPurge} onClick={purge}>{busy === 'purge' ? '正在核对并清理…' : '清理到期原文'}</button>
    </> : null}
    {status?.status === 'error' ? <p role="alert" className="action-error">原文仓签名状态或独立密钥异常。系统不会自动重建或覆盖；请运行 <span className="mono">siq-agent-security status</span> 检查服务状态。</p> : null}
    {notice ? <p className={notice.error ? 'action-error' : 'action-success'} role={notice.error ? 'alert' : 'status'}>{notice.text}</p> : null}
  </div>;
}
