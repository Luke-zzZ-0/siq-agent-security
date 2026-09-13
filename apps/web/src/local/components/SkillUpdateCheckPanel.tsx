import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import { localApi } from '../api';
import { useLocalSession } from '../session';
import { updateCheckErrorText, type SkillUpdateCheckResult } from '../skillUpdateCheck';

export default function SkillUpdateCheckPanel({ installId, disabled = false }: { installId: string; disabled?: boolean }) {
  const { actorId } = useLocalSession();
  const [url, setUrl] = useState('');
  const [result, setResult] = useState<SkillUpdateCheckResult | null>(null);
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const active = useRef<AbortController | null>(null);
  useEffect(() => {
    active.current?.abort(); active.current = null; setBusy(false); setResult(null); setError('');
    return () => { active.current?.abort(); active.current = null; };
  }, [installId, disabled]);
  const cancel = () => { active.current?.abort(); active.current = null; setBusy(false); setError('检查已取消。'); };
  const check = async () => {
    if (active.current || disabled || !url.trim() || !actorId.trim()) return;
    const controller = new AbortController(); active.current = controller;
    setBusy(true); setResult(null); setError('');
    try {
      const data = await localApi.checkSkillUpdate(installId, { schema_version: 'local-skill-update-check/v1', remote_url: url.trim(), actor_id: actorId.trim() }, controller.signal);
      if (active.current === controller && !controller.signal.aborted) setResult(data);
    } catch (err) {
      if (active.current === controller && !controller.signal.aborted) setError(updateCheckErrorText(err));
    } finally {
      if (active.current === controller) { active.current = null; setBusy(false); }
    }
  };
  return <section className="panel import-panel" aria-label="检查 Skill 新版">
    <h3>检查新版</h3>
    <p className="page-desc">检查原 HTTPS ZIP 来源是否有内容变化，不会自动更新文件或权限。Git 来源暂不可用，本地目录没有可检查的上游。</p>
    <form onSubmit={(event) => { event.preventDefault(); void check(); }}>
      <div className="field">
      <label htmlFor="skill-update-source-url">原 HTTPS ZIP 下载链接</label>
      <input id="skill-update-source-url" type="url" required maxLength={4096} value={url} autoComplete="off" spellCheck={false}
        disabled={disabled || busy} placeholder="https://…/skill.zip" onChange={(event) => { setUrl(event.target.value); setResult(null); setError(''); }} />
      <p className="page-desc">使用首次导入时的链接。链接仅用于本次检查，不会保存。</p>
      </div>
      <div className="import-actions"><button className="btn" type="submit" disabled={disabled || busy || !url.trim() || !actorId.trim()}>检查新版</button>
        {busy ? <button className="btn" type="button" onClick={cancel}>取消检查</button> : null}</div>
    </form>
    {busy ? <p role="status">正在获取上游并比较内容…</p> : null}
    {error ? <p role="alert" className="action-error">{error}</p> : null}
    {result ? <div>
      <p role="status">{result.status === 'up_to_date' ? '上游内容与安装记录一致。' : `发现 ${result.content_changes_total} 项内容变化，需要你确认后更新。`}</p>
      <p className="page-desc">检查时间：{new Date(result.checked_at).toLocaleString()}。权限差异将在导入候选后审阅。</p>
      {result.content_changes_truncated ? <p>仅显示前 200 项变化。</p> : null}
      <ul>{result.content_changes.map((change) => <li key={change.path_digest}>{change.before === null ? '新增' : change.after === null ? '删除' : '修改'}：{change.path_display}</li>)}</ul>
      {result.requires_confirmation ? <div className="import-actions"><Link className="btn" to="/skill-imports">重新导入候选</Link><Link className="btn" to={`/skill-updates?install_id=${encodeURIComponent(installId)}`}>审阅候选并更新</Link></div> : null}
    </div> : null}
  </section>;
}
