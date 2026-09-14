import { useEffect, useMemo } from 'react';
import { useLocalSession } from './session';

export type LoadGuard = {
  <T>(load: () => Promise<T>): Promise<T | undefined>;
  invalidate(): void;
};

// An invalidated request loses ownership of both its value and its error.
export function createLoadGuard(): LoadGuard {
  let current = 0;
  const guard: LoadGuard = (load) => {
    const seq = ++current;
    return load().then(
      (value) => (seq === current ? value : undefined),
      (err: unknown) => {
        if (seq !== current) return undefined;
        throw err;
      },
    );
  };
  guard.invalidate = () => { current++; };
  return guard;
}

export function useLoadGuard(): LoadGuard {
  const { status, actorId } = useLocalSession();
  const connected = status !== null;
  const signer = status?.signing_public_key;
  const guard = useMemo(() => createLoadGuard(), [connected, signer, actorId]);
  // Includes route unmount, sign out, server identity and actor changes.
  // Invalidation also permits StrictMode's next setup to start a fresh load.
  useEffect(() => () => guard.invalidate(), [guard]);
  return guard;
}
