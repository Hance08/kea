import { useEffect, useMemo, useState } from 'react';
import GridLayout, { WidthProvider, type Layout } from 'react-grid-layout';
import { DEFAULT_STATE } from '../../lib/dashboard/defaults';
import { ALL_WIDGET_IDS, WIDGETS } from '../../lib/dashboard/registry';
import {
  loadDashboardState,
  reconcileWithRegistry,
  saveDashboardState,
} from '../../lib/dashboard/storage';
import type { DashboardState, GridItem, WidgetId } from '../../lib/dashboard/types';
import { EditModeHeader } from './EditModeHeader';
import { WidgetFrame } from './WidgetFrame';
import './grid.css';

const ResponsiveGrid = WidthProvider(GridLayout);
const COLS = 3;
const ROW_HEIGHT = 80;

export function Dashboard() {
  const [editing, setEditing] = useState(false);
  const [state, setState] = useState<DashboardState>(() => {
    const saved = loadDashboardState();
    return saved
      ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE)
      : DEFAULT_STATE;
  });

  useEffect(() => {
    const t = setTimeout(() => saveDashboardState(state), 500);
    return () => clearTimeout(t);
  }, [state]);

  const visibleItems = useMemo(
    () => state.layout.filter((g) => !state.hidden.includes(g.i)),
    [state.layout, state.hidden],
  );

  function hideWidget(id: WidgetId) {
    setState((s) => ({ ...s, hidden: [...s.hidden, id] }));
  }

  function onLayoutChange(next: Layout[]) {
    setState((s) => {
      const byId = new Map(next.map((n) => [n.i as WidgetId, n]));
      const updated = s.layout.map<GridItem>((g) => {
        const n = byId.get(g.i);
        return n ? { i: g.i, x: n.x, y: n.y, w: n.w, h: n.h } : g;
      });
      return { ...s, layout: updated };
    });
  }

  return (
    <div className="p-4">
      <EditModeHeader editing={editing} onToggle={() => setEditing((e) => !e)} />
      <div className="dashboard-grid">
        <ResponsiveGrid
          cols={COLS}
          rowHeight={ROW_HEIGHT}
          layout={visibleItems}
          isDraggable={editing}
          isResizable={editing}
          draggableHandle=".dashboard-drag-handle"
          onLayoutChange={onLayoutChange}
          margin={[16, 16]}
        >
          {visibleItems.map((item: GridItem) => {
            const meta = WIDGETS[item.i];
            const cfg = state.config[item.i] ?? meta.defaultConfig;
            const Comp = meta.component;
            return (
              <div
                key={item.i}
                className="rounded-lg border bg-card shadow-sm overflow-hidden"
              >
                <WidgetFrame meta={meta} editing={editing} onHide={() => hideWidget(item.i)}>
                  <Comp config={cfg} />
                </WidgetFrame>
              </div>
            );
          })}
        </ResponsiveGrid>
      </div>
    </div>
  );
}
