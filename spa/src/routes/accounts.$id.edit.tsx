import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id/edit')({
  component: AccountEditPage,
});

function AccountEditPage() {
  const { id } = Route.useParams();
  return <div>Edit {id}</div>;
}
