// Keep this thin — helpers that import routeTree (and thus the full app graph)
// belong in test-app.tsx so global setup does not pre-load modules before
// vi.mock() runs in individual test files.
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

// Node 25 exposes a built-in `localStorage` global that shadows jsdom's
// implementation but is not a real Storage. Detect by missing setItem and
// fall back to an in-memory Storage shim so tests behave consistently across
// Node versions.
if (typeof localStorage === 'undefined' || typeof localStorage.setItem !== 'function') {
  const store = new Map<string, string>();
  const shim: Storage = {
    get length() {
      return store.size;
    },
    clear() {
      store.clear();
    },
    getItem(key) {
      return store.has(key) ? (store.get(key) as string) : null;
    },
    key(index) {
      return Array.from(store.keys())[index] ?? null;
    },
    removeItem(key) {
      store.delete(key);
    },
    setItem(key, value) {
      store.set(key, String(value));
    },
  };
  Object.defineProperty(globalThis, 'localStorage', {
    configurable: true,
    value: shim,
  });
}

afterEach(() => {
  cleanup();
});
