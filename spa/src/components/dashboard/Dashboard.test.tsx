import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@testing-library/user-event';
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

describe('Dashboard edit mode', () => {
  it('toggles edit mode and reveals hide buttons', async () => {
    const user = userEvent.setup();
    renderDashboard();
    expect(screen.queryByLabelText(/hide net-worth-kpi/i)).toBeNull();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    expect(screen.getByLabelText(/hide net-worth-kpi/i)).toBeInTheDocument();
  });

  it('hides a widget when clicking its hide button', async () => {
    const user = userEvent.setup();
    const { container } = renderDashboard();
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/hide net-worth-kpi/i));
    await user.click(screen.getByRole('button', { name: /^done$/i }));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(7);
  });
});
