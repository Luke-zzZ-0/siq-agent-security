import { useEffect, useRef, useState } from 'react';
import { localApi, LocalApiError } from '../api';
import type { TaskActivityDetail } from '../taskActivities';
export default function ActivityExportButton({ detail }: { detail: TaskActivityDetail }) {
  const pending = useRef<AbortController | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  useEffect(() => () => { pending.current?.abort(); }, []);
  const download = async () => {
    if (pending.current) return;
    const controller = new AbortController();
    pending.current = controller; setBusy(true); setError('');
    try {
      const data = await localApi.taskActivityExport(detail.activity, detail.snapshot, controller.signal);
      if (controller.signal.aborted) return;
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2) + '\n'], { type: 'application/json' }));
      const link = document.createElement('a');
      link.href = url; link.download = `siq-activity-${detail.activity.activity_id}.json`;
      document.body.appendChild(link); link.click(); link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      if (!controller.signal.aborted) setError(e instanceof LocalApiError && e.status === 409 ? '活动记录已更新，请刷新详情后重新下载。'
        : e instanceof LocalApiError && e.status === 413 ? '活动记录超过导出上限，当前无法下载整组摘要。' : '摘要下载失败或内容无法通过校验，请稍后重试。');
    } finally {
      if (!controller.signal.aborted) { pending.current = null; setBusy(false); }
    }
  };
  return <section aria-label="活动摘要导出">
    <button className="btn" disabled={busy} onClick={() => void download()}>{busy ? '正在准备摘要…' : '下载脱敏回执摘要'}</button>
    <p>下载整个活动的回执摘要，不含参数原文、效果材料或 Skill 版本。独立签名用于核对摘要来源，不代表任务已完成。</p>
    {error ? <p role="alert" className="action-error">{error}</p> : null}
  </section>;
}
