import { describe, expect, it } from 'vitest';
import { createLoadGuard } from './staleGuard';

describe('createLoadGuard 陈旧响应守卫', () => {
  it('lets the latest load apply its result', async () => {
    const guard = createLoadGuard();
    await expect(guard(() => Promise.resolve('first'))).resolves.toBe('first');
    await expect(guard(() => Promise.resolve('second'))).resolves.toBe('second');
  });

  it('suppresses a stale response that resolves after a newer load', async () => {
    const guard = createLoadGuard();
    let releaseOld: (value: string) => void = () => {};
    const old = guard(() => new Promise<string>((resolve) => { releaseOld = resolve; }));
    const fresh = guard(() => Promise.resolve('fresh'));
    releaseOld('stale-value');
    await expect(fresh).resolves.toBe('fresh');
    await expect(old).resolves.toBeUndefined();
  });

  it('suppresses a stale load failure instead of letting it clobber fresh state', async () => {
    const guard = createLoadGuard();
    let rejectOld: (err: Error) => void = () => {};
    const old = guard(() => new Promise<string>((_, reject) => { rejectOld = reject; }));
    const fresh = guard(() => Promise.resolve('fresh'));
    rejectOld(new Error('slow request died late'));
    await expect(fresh).resolves.toBe('fresh');
    await expect(old).resolves.toBeUndefined();
  });

  it('still surfaces errors from the latest load', async () => {
    const guard = createLoadGuard();
    await expect(guard(() => Promise.reject(new Error('boom')))).rejects.toThrow('boom');
  });

  it('resumes applying results once the stale load is drained', async () => {
    const guard = createLoadGuard();
    let releaseOld: (value: string) => void = () => {};
    const old = guard(() => new Promise<string>((resolve) => { releaseOld = resolve; }));
    await expect(guard(() => Promise.resolve('newer'))).resolves.toBe('newer');
    releaseOld('late');
    await expect(old).resolves.toBeUndefined();
    await expect(guard(() => Promise.resolve('latest'))).resolves.toBe('latest');
  });
  it('invalidates pending success and failure when their owner leaves', async () => {
    for (const reject of [false, true]) {
      const guard = createLoadGuard();
      let finish: () => void = () => {};
      const pending = guard(() => new Promise<string>((resolve, fail) => {
        finish = () => reject ? fail(new Error('old session')) : resolve('old session');
      }));
      guard.invalidate();
      finish();
      await expect(pending).resolves.toBeUndefined();
      await expect(guard(() => Promise.resolve('new session'))).resolves.toBe('new session');
    }
  });

});
