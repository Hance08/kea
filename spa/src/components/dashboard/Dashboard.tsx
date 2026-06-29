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
import { useMediaQuery } from '../../lib/hooks/useMediaQuery';
import { EditModeHeader } from './EditModeHeader';
import { WidgetFrame } from './WidgetFrame';
import './grid.css';

const ResponsiveGrid = WidthProvider(GridLayout);
const COLS = 3;
const ROW_HEIGHT = 96;

export function Dashboard() {
  const [editing, setEditing] = useState(false);
  const [state, setState] = useState<DashboardState>(() => {
    const saved = loadDashboardState();
    return saved ? reconcileWithRegistry(saved, ALL_WIDGET_IDS, DEFAULT_STATE) : DEFAULT_STATE;
  });

  const isMobile = useMediaQuery('(max-width: 640px)');
  const cols = isMobile ? 1 : COLS;
  useEffect(() => {
    if (isMobile) setEditing(false);
  }, [isMobile]);

  useEffect(() => {
    const t = setTimeout(() => saveDashboardState(state), 500);
    return () => clearTimeout(t);
  }, [state]);

  function toggleEditing() {
    setEditing((wasEditing) => {
      // Exiting edit mode: flush pending state immediately so a fast tab-close
      // can't lose the user's last tweak (the debounced save above might miss).
      if (wasEditing) saveDashboardState(state);
      return !wasEditing;
    });
  }

  const visibleItems = useMemo(
    () => state.layout.filter((g) => !state.hidden.includes(g.i)),
    [state.layout, state.hidden],
  );

  function hideWidget(id: WidgetId) {
    setState((s) => ({ ...s, hidden: [...s.hidden, id] }));
  }

  function setConfig(id: WidgetId, cfg: unknown) {
    setState((s) => ({ ...s, config: { ...s.config, [id]: cfg } }));
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
        onToggle={toggleEditing}
        hidden={state.hidden}
        onAdd={addWidget}
        onReset={resetToDefaults}
        isMobile={isMobile}
      />
      {visibleItems.length === 0 ? (
        <EmptyDashboard hasHidden={state.hidden.length > 0} onEnterEdit={() => setEditing(true)} />
      ) : (
        <div className="dashboard-grid">
          <ResponsiveGrid
            cols={cols}
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
                  <WidgetFrame
                    meta={meta}
                    editing={editing}
                    config={cfg}
                    onConfigChange={(c) => setConfig(item.i, c)}
                    onHide={() => hideWidget(item.i)}
                  >
                    <Comp config={cfg} />
                  </WidgetFrame>
                </div>
              );
            })}
          </ResponsiveGrid>
        </div>
      )}
    </div>
  );
}

function EmptyDashboard({
  hasHidden,
  onEnterEdit,
}: { hasHidden: boolean; onEnterEdit: () => void }) {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed py-16 text-center">
      <p className="text-sm text-muted-foreground">
        {hasHidden ? 'All widgets are hidden. Add one back to get started.' : 'No widgets to show.'}
      </p>
      <button
        type="button"
        onClick={onEnterEdit}
        className="rounded-md border px-3 py-1.5 text-sm font-medium hover:bg-muted"
      >
        {hasHidden ? 'Add widgets' : 'Edit dashboard'}
      </button>
    </div>
  );
}
