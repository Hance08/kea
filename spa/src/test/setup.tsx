import '@testing-library/jest-dom/vitest';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
// Re-export a TestApp helper for routing-aware tests.
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router';
import { cleanup } from '@testing-library/react';
import { afterEach } from 'vitest';
import { ServerConfigProvider } from '../lib/server-config';
import { routeTree } from '../routeTree.gen';

afterEach(() => {
  cleanup();
});

export function makeTestApp(initialPath: string) {
  const history = createMemoryHistory({ initialEntries: [initialPath] });
  const router = createRouter({ routeTree, history });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return (
    <QueryClientProvider client={queryClient}>
      <ServerConfigProvider fallback={<div>Loading…</div>}>
        {() => <RouterProvider router={router} />}
      </ServerConfigProvider>
    </QueryClientProvider>
  );
}
