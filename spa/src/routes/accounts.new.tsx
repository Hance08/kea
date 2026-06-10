import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/new')({
  component: AccountNewPage,
});

function AccountNewPage() {
  return <div>New account</div>;
}
