import { useEffect, useState } from 'react';
import { localApi, LocalApiError } from '../api';
import type { TaskActivityDetail } from '../taskActivities';
import type { ActivitySources } from '../taskSources';
export default function ActivitySourcesPanel({ detail }: { detail: TaskActivityDetail }) {
  const [open, setOpen] = useState(false);
  const [data, setData] = useState<ActivitySources>();
  const [error, setError] = useState('');
  useEffect(() => {
    if (!open) return;
    let active = true;
    const controller = new AbortController();
    setData(undefined); setError('');
    localApi.taskActivitySources(detail, controller.signal).then((value) => { if (active) setData(value); }).catch((e) => {
      if (active) setError(e instanceof LocalApiError && e.status === 409 ? '活动记录已更新，请刷新详情后重新查看来源。'
        : e instanceof LocalApiError && e.status === 413 ? '本页历史关联超过查询上限，当前无法展示来源。' : '历史来源当前不可读取或无法核验，请关闭后重试。');
    });
    return () => { active = false; controller.abort(); };
  }, [open, detail]);
  return <section aria-label="历史 Skill 来源">
    <button className="btn" aria-expanded={open} onClick={() => { setData(undefined); setError(''); setOpen(!open); }}>{open ? '关闭历史 Skill 来源' : '查看历史 Skill 来源'}</button>
    {open ? <>
      <p>本页回执所引用的历史授权来源。声明版本与分析摘要不证明该 Skill 实际执行，也不代表当前权限或安装状态。</p>
      {error ? <p role="alert" className="action-error">{error}</p> : !data ? <p role="status">正在核对历史来源…</p> : null}
      {data?.items.map((row) => <div key={row.seq} style={{ overflowWrap: 'anywhere' }}>
        <h3>回执 #{row.seq}</h3>
        {row.source ? <>
          <p>{row.source.skill_name} · 声明版本：{row.source.declared_version ?? '未声明'}</p>
          <p>历史授权来源已核验。授权：{row.source.grant_id}；准入：{row.source.admission_id}</p>
          <p className="mono">准入内容摘要：{row.source.content_hash}</p>
          {row.source.import ? <><p>导入记录：{row.source.import.import_id}</p><p className="mono">制品摘要：{row.source.import.artifact_digest}</p><p className="mono">分析摘要：{row.source.import.analysis_sha256}</p></> : null}
        </> : <p>{row.status === 'unattributed' ? '未归属回执，无法确定历史 Skill 来源。' : '缺少可信关联或来源材料无法核验，历史 Skill 来源未知。'}</p>}
      </div>)}
      {data && data.items.length === 0 ? <p>本页没有回执来源。</p> : null}
    </> : null}
  </section>;
}
