import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { ServerConfigContext } from '../../lib/server-config';
import type { ServerConfig } from '../../lib/types';
import { Dashboard } from './Dashboard';

const TEST_CONFIG: ServerConfig = {
  defaults: { currency: 'USD' },
  display: { hide_decimals: false },
};

function renderDashboard() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ServerConfigContext.Provider value={TEST_CONFIG}>
        <Dashboard />
      </ServerConfigContext.Provider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  localStorage.clear();
  localStorage.setItem('kea.activeLedger', 'main');
});

describe('Dashboard', () => {
  it('renders 8 grid items by default (one per registered widget)', () => {
    const { container } = renderDashboard();
    // react-grid-layout assigns `react-grid-item` class to each item.
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
  });
});
