import { useEffect, useState } from 'react';
import { localApi } from '../api';
import type { EffectEvidenceSummary } from '../effectEvidence';
const independence: Record<string, string> = { self_reported: '执行方自报', host_independent: '宿主独立观测', external_independent: '外部独立观测', unknown: '未知' };
const coverage: Record<string, string> = { full: '完整覆盖', partial: '部分覆盖', unknown: '覆盖未知' };
const execution: Record<string, string> = { requested: '已请求', started: '已开始', completed: '观测到完成', failed: '观测到失败', unknown: '未知' };
const result: Record<string, string> = { expected: '符合预期', unexpected: '不符合预期', conflicting: '存在冲突', unknown: '未知' };
export default function EffectEvidenceDetails({ id, taskId, close }: { id: string; taskId: string; close: () => void }) {
  const [state, setState] = useState<{ key: string; data?: EffectEvidenceSummary; error?: string }>();
  const key = JSON.stringify([id, taskId]);
  useEffect(() => {
    let active = true;
    const controller = new AbortController();
    localApi.effectEvidence(id, taskId, controller.signal).then((data) => {
      if (active) setState({ key, data });
    }).catch(() => { if (active) setState({ key, error: '这份证据当前不可读取或无法通过校验，请稍后重新打开。' }); });
    return () => { active = false; controller.abort(); };
  }, [id, taskId, key]);
  const current = state?.key === key ? state : undefined;
  const data = current?.data;
  return <section aria-label="效果证据详情">
    <h3>证据 {id}</h3>
    <button className="btn" onClick={close}>关闭证据详情</button>
    {!current ? <p role="status">正在读取证据…</p> : null}
    {current?.error ? <p role="alert" className="action-error">{current.error}</p> : null}
    {data ? <>
      <p>来源：{data.sourceId}（{data.sourceType}）</p>
      <p>{independence[data.independence]} · {coverage[data.coverage]}</p>
      <p>执行状态：{execution[data.executionState]}；观测结果：{result[data.result]}</p>
      <p>效果类型：{data.effectType}；观测时间：{data.observedAt}</p>
      <p>动作引用：{data.actionId}；裁决回执：{data.receiptId}</p>
      <p className="mono" style={{ overflowWrap: 'anywhere' }}>资源摘要：{data.resourceRef}</p>
      <p className="mono" style={{ overflowWrap: 'anywhere' }}>证据摘要：{data.digest}</p>
      {data.finding ? <p className="action-error">安全事件：{data.finding}</p> : null}
      <p>证据由本地服务验签。单份证据的完成状态不能替代任务效果核验结论。</p>
    </> : null}
  </section>;
}
