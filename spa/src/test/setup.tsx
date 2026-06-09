// Keep this thin — helpers that import routeTree (and thus the full app graph)
// belong in test-app.tsx so global setup does not pre-load modules before
// vi.mock() runs in individual test files.
import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';

afterEach(() => {
  cleanup();
});
