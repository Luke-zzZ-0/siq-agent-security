import { useEffect, useState } from 'react';
import { localApi } from '../api';
import type { RawContentStatus } from '../rawTaskContent';

export default function RawContentStatusIndicator() {
  const [state, setState] = useState<{ status?: RawContentStatus; unavailable?: boolean }>({});

  useEffect(() => {
    let active = true;
    let controller: AbortController | undefined;
    const refresh = () => {
      controller?.abort();
      controller = new AbortController();
      const request = controller;
      localApi.rawContentStatus(request.signal).then((status) => {
        if (active) setState({ status });
      }).catch(() => {
        if (active && !request.signal.aborted) setState({ unavailable: true });
      });
    };
    refresh();
    const timer = window.setInterval(refresh, 30_000);
    const onFocus = () => refresh();
    window.addEventListener('focus', onFocus);
    window.addEventListener('siq:raw-content-status-changed', onFocus);
    return () => {
      active = false;
      controller?.abort();
      window.clearInterval(timer);
      window.removeEventListener('focus', onFocus);
      window.removeEventListener('siq:raw-content-status-changed', onFocus);
    };
  }, []);

  const status = state.status;
  const label = state.unavailable ? '原文状态不可用'
    : !status ? '正在读取原文状态'
    : status.status === 'disabled' ? '原文记录关闭'
    : status.status === 'error' ? '原文仓状态异常'
    : '原文仓已启用 · 按任务授权';
  const tone = status?.status === 'ready' ? ' raw-content-ready'
    : status?.status === 'error' || state.unavailable ? ' raw-content-error' : '';

  return <span className={`topbar-phase raw-content-status${tone}`} role="status" title={status?.status === 'ready' ? '仓已启用不表示任何任务正在采集；每个任务仍需单独授权。' : undefined}>
    <span aria-hidden="true" />
    {label}
  </span>;
}
