import { Sidebar } from '@/components/Sidebar';
import { Outlet, createRootRoute } from '@tanstack/react-router';

export const Route = createRootRoute({
  component: RootLayout,
});

function RootLayout() {
  return (
    <div className="flex min-h-screen">
      <Sidebar />
      {/*
        min-w-0 is critical: without it, a flex child's min-width is
        auto = min-content of its children, so wide content (e.g., the
        transactions table's fixed-width grid) propagates out and
        forces the page to scroll horizontally. min-w-0 lets main
        shrink to the available flex space; children handle their own
        overflow (e.g., the table's overflow-x-auto wrapper).
      */}
      <main className="min-w-0 flex-1 p-8">
        <Outlet />
      </main>
    </div>
  );
}
