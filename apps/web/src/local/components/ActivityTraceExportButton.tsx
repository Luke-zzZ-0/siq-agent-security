import { useEffect, useRef, useState } from 'react';
import { localApi, LocalApiError } from '../api';
import type { TaskActivityDetail } from '../taskActivities';

export default function ActivityTraceExportButton({ detail }: { detail: TaskActivityDetail }) {
  const pending = useRef<AbortController | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [result, setResult] = useState('');
  useEffect(() => () => { pending.current?.abort(); }, []);
  const download = async () => {
    if (pending.current) return;
    const controller = new AbortController();
    pending.current = controller;
    setBusy(true);
    setError('');
    setResult('');
    try {
      const data = await localApi.taskActivityTraceExport(detail.activity, detail.snapshot, controller.signal);
      if (controller.signal.aborted) return;
      const url = URL.createObjectURL(new Blob([JSON.stringify(data, null, 2) + '\n'], { type: 'application/json' }));
      const link = document.createElement('a');
      link.href = url;
      link.download = `siq-trace-${detail.activity.activity_id}.json`;
      document.body.appendChild(link);
      link.click();
      link.remove();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      setResult(data.incomplete === true ? '追溯包已下载；其中包含材料不完整的明确标记。' : '追溯包已下载；其中的效果结论已核验。');
    } catch (e) {
      if (!controller.signal.aborted) {
        setError(e instanceof LocalApiError && e.status === 409 ? '活动或效果记录已更新，请刷新详情后重新下载。'
          : e instanceof LocalApiError && e.status === 413 ? '活动或历史来源超过追溯包上限，请缩小活动范围。'
            : '追溯包下载失败或内容无法通过格式与关联校验，请稍后重试。');
      }
    } finally {
      if (!controller.signal.aborted) {
        pending.current = null;
        setBusy(false);
      }
    }
  };
  return <section aria-label="完整追溯包导出">
    <button className="btn btn-primary" disabled={busy} onClick={() => void download()}>{busy ? '正在准备追溯包…' : '下载完整脱敏追溯包'}</button>
    <p>包含回执摘要、历史 Skill 来源状态、效果结论和被引用的证据元数据。浏览器只核对格式与关联；签名仍需使用外部可信公钥验证。</p>
    {result ? <p role="status">{result}</p> : null}
    {error ? <p role="alert" className="action-error">{error}</p> : null}
  </section>;
}
