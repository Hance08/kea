import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/transactions/$id')({
  component: () => <div>Transaction Detail (Task 14)</div>,
});
