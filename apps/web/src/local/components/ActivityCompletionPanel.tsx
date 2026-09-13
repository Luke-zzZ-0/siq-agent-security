import EffectEvidenceDetails from './EffectEvidenceDetails';
import { useEffect, useState } from 'react';
import { LocalApiError, localApi } from '../api';
import { completionLabel, completionReason, type ActivityCompletion } from '../taskCompletion';
import type { TaskActivityDetail } from '../taskActivities';

export default function ActivityCompletionPanel({ detail }: { detail: TaskActivityDetail }) {
  const [selected, setSelected] = useState<{ detail: TaskActivityDetail; id: string; trigger: HTMLButtonElement }>();
  const [state, setState] = useState<{ detail: TaskActivityDetail; value?: ActivityCompletion; error?: string }>();
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    localApi.taskActivityCompletion(detail.activity, detail.view, detail.snapshot, controller.signal).then((value) => {
      if (active) setState({ detail, value });
    }).catch((error: unknown) => {
      if (active) setState({ detail, error: error instanceof LocalApiError && error.status === 409
        ? '活动快照已变化，请刷新详情后重新核验。' : '效果证据当前不可用，请刷新详情重试；不能确认效果已核验。' });
    });
    return () => { active = false; controller.abort(); };
  }, [detail]);
  const current = state?.detail === detail ? state : undefined;
  const value = current?.value;
  const result = value?.result;
  return <section aria-label="实际效果核验">
    <h2>实际效果核验</h2>
    {!current ? <p role="status">正在核对效果证据…</p> : null}
    {current?.error ? <p role="alert" className="action-error">{current.error}</p> : null}
    {value?.reason_code === 'attribution_unknown' ? <p>缺少可信任务归属，无法核验实际效果。</p> : null}
    {value?.reason_code === 'intent_missing' ? <p>未找到对应的签名意图，无法核验实际效果。</p> : null}
    {result ? <>
      <p role="status"><strong>{completionLabel[result.status]}</strong> · {completionReason(result.reason_code)}</p>
      {result.requirements.map((item) => <div key={item.requirement_id}>
        <p><strong>{item.requirement_id}</strong>：{completionLabel[item.status]}。{completionReason(item.reason_code)}</p>
        <p>证据引用：{item.evidence_ids.length ? item.evidence_ids.join('、') : '暂无'}</p>
      </div>)}
      {result.incident_ids.length ? <p className="action-error">安全事件引用：{result.incident_ids.join('、')}</p> : null}
    </> : null}
    {result ? <div className="toolbar">
      {[...new Set([...result.requirements.flatMap((item) => item.evidence_ids), ...result.incident_ids])].map((id) => <button className="btn" key={id} onClick={(event) => setSelected({ detail, id, trigger: event.currentTarget })}>查看证据 {id}</button>)}
    </div> : null}
    {result && selected?.detail === detail && [...result.requirements.flatMap((item) => item.evidence_ids), ...result.incident_ids].includes(selected.id)
      ? <EffectEvidenceDetails key={selected.id} id={selected.id} taskId={result.task_id} close={() => { selected.trigger.focus(); setSelected(undefined); }} /> : null}
    {value ? <p>核验时间：{value.evaluated_at}。结论仅针对本次读取的证据。</p> : null}
  </section>;
}
