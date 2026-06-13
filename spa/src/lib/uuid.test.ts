import { afterEach, describe, expect, test, vi } from 'vitest';
import { safeRandomUUID } from './uuid';

const UUID_V4_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

describe('safeRandomUUID', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  test('returns a valid UUIDv4', () => {
    expect(safeRandomUUID()).toMatch(UUID_V4_RE);
  });

  test('returns different values on consecutive calls', () => {
    expect(safeRandomUUID()).not.toBe(safeRandomUUID());
  });

  test('falls back to getRandomValues when crypto.randomUUID is unavailable', () => {
    const originalCrypto = globalThis.crypto;
    vi.stubGlobal('crypto', {
      getRandomValues: originalCrypto.getRandomValues.bind(originalCrypto),
      randomUUID: undefined,
    });

    expect(safeRandomUUID()).toMatch(UUID_V4_RE);
  });
});
