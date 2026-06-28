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
    return saved ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE) : DEFAULT_STATE;
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

  function addWidget(id: WidgetId) {
    setState((s) => {
      const next = { ...s, hidden: s.hidden.filter((h) => h !== id) };
      // If the widget has no layout entry (edge case), append it.
      if (!next.layout.find((g) => g.i === id)) {
        const def = DEFAULT_STATE.layout.find((g) => g.i === id);
        const maxY = next.layout.reduce((m, g) => Math.max(m, g.y + g.h), 0);
        next.layout = [
          ...next.layout,
          { i: id, x: def?.x ?? 0, y: maxY, w: def?.w ?? 1, h: def?.h ?? 1 },
        ];
      }
      return next;
    });
  }

  function resetToDefaults() {
    setState(DEFAULT_STATE);
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
      <EditModeHeader
        editing={editing}
        onToggle={() => setEditing((e) => !e)}
        hidden={state.hidden}
        onAdd={addWidget}
        onReset={resetToDefaults}
      />
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
              <div key={item.i} className="rounded-lg border bg-card shadow-sm overflow-hidden">
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
