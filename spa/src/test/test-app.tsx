import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { RouterProvider, createMemoryHistory, createRouter } from '@tanstack/react-router';
import { ServerConfigProvider } from '../lib/server-config';
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
