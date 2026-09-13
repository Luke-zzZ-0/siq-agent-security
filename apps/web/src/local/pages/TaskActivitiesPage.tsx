import { useEffect, useState, type FormEvent } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import PageHeader from '@/components/PageHeader';
import SimpleTable, { type TableColumn } from '@/components/SimpleTable';
import { LocalApiError, localApi } from '../api';
import { platformLabel } from '../format';
import { validActivityFilter, type ActivityFilters, type ActivityView, type TaskActivityItem, type TaskActivityPage, type TaskActivitySearch } from '../taskActivities';

const columns: TableColumn<TaskActivityItem>[] = [
  { key: 'task', header: '任务', render: (r) => r.binding?.task_id ?? '未归属活动' },
  { key: 'platform', header: '平台', render: (r) => r.binding ? platformLabel(r.binding.platform) : '未知' },
  { key: 'agent', header: '智能体', render: (r) => r.binding?.agent_id ?? '缺少可信绑定' },
  { key: 'session', header: '会话', render: (r) => r.binding?.session_id ?? '未知' },
  { key: 'count', header: '回执数', render: (r) => String(r.receipt_count) },
  { key: 'range', header: '回执序号', render: (r) => `${r.first_seq}–${r.last_seq}` },
];
const filterNames = ['platform', 'agent_id', 'session_id', 'task_id', 'q'] as const;
const readFilters = (params: URLSearchParams): ActivityFilters => Object.fromEntries(filterNames.map((name) => {
  const value = params.get(name) ?? '';
  return [name, validActivityFilter(value) ? value : ''];
})) as unknown as ActivityFilters;
const activityParams = (view: ActivityView, filters: ActivityFilters, extra: Record<string, string> = {}) => {
  const next = new URLSearchParams({ view });
  for (const name of filterNames) if (filters[name]) next.set(name, filters[name]);
  for (const [name, value] of Object.entries(extra)) if (value) next.set(name, value);
  return next;
};
export default function TaskActivitiesPage() {
  const [params, setParams] = useSearchParams();
  const view: ActivityView = params.get('view') === 'unassigned' ? 'unassigned' : 'tasks';
  const rawOffset = params.get('offset') ?? '0';
  const offset = /^(0|[1-9][0-9]{0,5})$/.test(rawOffset) ? Number(rawOffset) : 0;
  const snapshot = params.get('snapshot') || undefined;
  const filters = readFilters(params);
  const filterKey = JSON.stringify(filters);
  const filtered = filterNames.some((name) => filters[name] !== '');
  const [retry, setRetry] = useState(0);
  const [filterError, setFilterError] = useState('');
  const [state, setState] = useState<{ key: string; data?: TaskActivityPage | TaskActivitySearch; error?: string }>({ key: '' });
  const key = JSON.stringify([view, offset, snapshot, filterKey, retry]);
  const current = state.key === key ? state : undefined;
  const loading = !current?.data && !current?.error;
  useEffect(() => {
    const controller = new AbortController();
    let active = true;
    const request = filtered
      ? localApi.taskActivitySearch(view, offset, filters, snapshot, controller.signal)
      : localApi.taskActivities(view, offset, snapshot, controller.signal);
    request.then((data) => {
      if (active) setState({ key, data });
    }).catch((error: unknown) => {
      if (!active) return;
      const message = error instanceof LocalApiError && error.status === 409
        ? '活动记录已更新，请点击“刷新活动”重新加载。'
        : '暂时无法读取可信活动记录。请确认本地服务已启动，再刷新重试。';
      setState({ key, error: message });
    });
    return () => { active = false; controller.abort(); };
  }, [view, offset, snapshot, filterKey, filtered, key]);
  const data = current?.data;
  const navigate = (next: number) => {
    if (!data) return;
    setParams(activityParams(view, filters, { offset: String(next), snapshot: data.snapshot }));
  };
  const applyFilters = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const next = Object.fromEntries(filterNames.map((name) => [name, String(form.get(name) ?? '').trim()])) as unknown as ActivityFilters;
    if (filterNames.some((name) => !validActivityFilter(next[name]))) {
      setFilterError('筛选条件不能包含控制字符，且每项最多 256 个字符。');
      return;
    }
    setFilterError('');
    setParams(activityParams(view, next));
  };
  return <section>
    <PageHeader kicker="本机活动" icon="audit" title="任务活动"
      description="按可信绑定查看任务与会话。回执记录裁决过程，允许执行不表示实际效果已核验。"
      connection={loading ? 'loading' : current?.error ? 'disconnected' : 'connected'} connectionError={current?.error}
      actions={<button className="btn btn-primary" disabled={loading} onClick={() => { setParams(activityParams(view, filters)); setRetry((v) => v + 1); }}>刷新活动</button>} />
    <div className="card">
      <div className="toolbar">
        <button className={view === 'tasks' ? 'btn btn-primary' : 'btn'} aria-pressed={view === 'tasks'} onClick={() => setParams(activityParams('tasks', filters))}>已归属任务</button>
        <button className={view === 'unassigned' ? 'btn btn-primary' : 'btn'} aria-pressed={view === 'unassigned'} onClick={() => setParams(activityParams('unassigned', filters))}>未归属活动</button>
        <Link to="/receipts">查看原始回执列表</Link>
      </div>
      <form key={filterKey} onSubmit={applyFilters} aria-label="筛选任务活动">
        <div className="form-row activity-filters">
          <div className="field field-flush field-grow"><label htmlFor="activity-query">关键词</label><input id="activity-query" name="q" maxLength={256} defaultValue={filters.q} placeholder="任务、智能体、会话或平台标识" /></div>
          <div className="field field-flush"><label htmlFor="activity-platform">平台</label><input id="activity-platform" name="platform" maxLength={256} defaultValue={filters.platform} list="activity-platforms" placeholder="例如 hermes" /><datalist id="activity-platforms"><option value="openclaw" /><option value="hermes" /><option value="workbuddy" /></datalist></div>
          <div className="field field-flush"><label htmlFor="activity-agent">智能体</label><input id="activity-agent" name="agent_id" maxLength={256} defaultValue={filters.agent_id} /></div>
          <div className="field field-flush"><label htmlFor="activity-session">会话</label><input id="activity-session" name="session_id" maxLength={256} defaultValue={filters.session_id} /></div>
          <div className="field field-flush"><label htmlFor="activity-task">任务</label><input id="activity-task" name="task_id" maxLength={256} defaultValue={filters.task_id} /></div>
        </div>
        <div className="toolbar">
          <button className="btn btn-primary" type="submit">应用筛选</button>
          <button className="btn" type="button" disabled={!filtered} onClick={() => { setFilterError(''); setParams(activityParams(view, { platform: '', agent_id: '', session_id: '', task_id: '', q: '' })); }}>清空筛选</button>
          {filtered ? <span role="status">已按标识字段筛选，共 {data?.total ?? '…'} 项。</span> : null}
        </div>
        <p>关键词只搜索任务、智能体、会话和平台标识，不搜索参数原文或 Skill 内容。</p>
        {filterError ? <p role="alert" className="action-error">{filterError}</p> : null}
      </form>
      {view === 'unassigned' ? <p>这些记录缺少完整的任务或主体绑定，系统不会根据时间或工具名称猜测归属。</p> : null}
      {current?.error ? <p className="action-error" role="alert">{current.error}</p> : null}
      {data ? <p role="status">当前回执前缀验签通过。{data.history_integrity === 'verified' ? '已与独立检查点核对历史完整性。' : data.history_integrity === 'failed' ? '历史完整性核对失败，请检查证据记录。' : '尚无法确认完整历史是否存在缺失。'}结果证据需单独核验。</p> : null}
      <SimpleTable columns={[...columns, { key: 'detail', header: '记录', render: (r) => <Link to={`/activities/${r.activity_id}?${activityParams(view, filters, { snapshot: data?.snapshot ?? '', from: String(offset) })}`}>查看活动记录</Link> }]} rows={data?.items ?? []} rowKey={(r) => r.activity_id}
        emptyText={loading ? '正在读取活动…' : current?.error ? '活动记录当前不可用。' : filtered ? '没有匹配当前筛选条件的活动。' : view === 'tasks' ? '尚无已归属任务，可切换查看未归属活动。' : '没有未归属活动。'} />
      {data ? <div className="toolbar">
        <button className="btn" disabled={offset === 0} onClick={() => navigate(Math.max(0, offset - 50))}>上一页</button>
        <span>共 {data.total} 项</span>
        <button className="btn" disabled={data.next_offset === null} onClick={() => { if (data.next_offset !== null) navigate(data.next_offset); }}>下一页</button>
      </div> : null}
    </div>
  </section>;
}
