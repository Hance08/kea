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
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: (query: string) => ({
      matches: false,
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
      dispatchEvent: () => false,
    }),
  });
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

describe('Dashboard add widget', () => {
  it('re-adds a hidden widget via the Add menu', async () => {
    const user = userEvent.setup();
    const { container } = renderDashboard();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/hide net-worth-kpi/i));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(7);
    await user.click(screen.getByRole('button', { name: /^add widget$/i }));
    await user.click(screen.getByRole('menuitem', { name: /^net worth$/i }));
    expect(container.querySelectorAll('.react-grid-item').length).toBe(8);
  });
});

describe('Dashboard widget config', () => {
  it('saves per-widget config changes to localStorage', async () => {
    const user = userEvent.setup();
    renderDashboard();
    await user.click(screen.getByRole('button', { name: /^edit$/i }));
    await user.click(screen.getByLabelText(/configure recent-transactions/i));
    const select = await screen.findByRole('combobox', { name: /number of transactions/i });
    await user.selectOptions(select, '20');
    // Storage save is debounced 500ms.
    await new Promise((r) => setTimeout(r, 600));
    const raw = localStorage.getItem('kea.dashboard.main');
    expect(raw).toBeTruthy();
    const parsed = JSON.parse(raw as string);
    expect(parsed.config['recent-transactions']).toEqual({ limit: 20 });
  });
});
