import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router';
import type { ReactNode } from 'react';
import { ServerConfigContext, ServerConfigProvider } from '../lib/server-config';
import type { ServerConfig } from '../lib/types';
import { routeTree } from '../routeTree.gen';

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

// Test helper for component-level tests that render outside makeTestApp.
// Wraps children with a stubbed ServerConfig so useServerConfig/useAmountFormat
// can be called without standing up the full /api/config fetch stack.
export function withServerConfig(children: ReactNode, override?: Partial<ServerConfig>) {
  const config: ServerConfig = {
    defaults: { currency: 'USD', ...(override?.defaults ?? {}) },
    display: { hide_decimals: false, ...(override?.display ?? {}) },
  };
  return <ServerConfigContext.Provider value={config}>{children}</ServerConfigContext.Provider>;
}
