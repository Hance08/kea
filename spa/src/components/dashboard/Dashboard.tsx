import { useEffect, useMemo, useState } from 'react';
import GridLayout, { WidthProvider } from 'react-grid-layout';
import { DEFAULT_STATE } from '../../lib/dashboard/defaults';
import { ALL_WIDGET_IDS, WIDGETS } from '../../lib/dashboard/registry';
import {
  loadDashboardState,
  reconcileWithRegistry,
  saveDashboardState,
} from '../../lib/dashboard/storage';
import type { DashboardState, GridItem } from '../../lib/dashboard/types';
import './grid.css';

const ResponsiveGrid = WidthProvider(GridLayout);

const COLS = 3;
const ROW_HEIGHT = 80;

export function Dashboard() {
  const [state, _setState] = useState<DashboardState>(() => {
    const saved = loadDashboardState();
    return saved ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE) : DEFAULT_STATE;
  });

  // Debounced save on state change.
  useEffect(() => {
    const t = setTimeout(() => saveDashboardState(state), 500);
    return () => clearTimeout(t);
  }, [state]);

  const visibleItems = useMemo(
    () => state.layout.filter((g) => !state.hidden.includes(g.i)),
    [state.layout, state.hidden],
  );

  return (
    <div className="dashboard-grid p-4">
      <ResponsiveGrid
        cols={COLS}
        rowHeight={ROW_HEIGHT}
        layout={visibleItems}
        isDraggable={false}
        isResizable={false}
        margin={[16, 16]}
      >
        {visibleItems.map((item: GridItem) => {
          const meta = WIDGETS[item.i];
          const cfg = state.config[item.i] ?? meta.defaultConfig;
          const Comp = meta.component;
          return (
            <div
              key={item.i}
              className="rounded-lg border bg-card p-3 shadow-sm overflow-hidden"
            >
              <Comp config={cfg} />
            </div>
          );
        })}
      </ResponsiveGrid>
    </div>
  );
}
