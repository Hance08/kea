import { createFileRoute } from '@tanstack/react-router';

export const Route = createFileRoute('/accounts/$id/')({
  component: AccountDetailPage,
});

function AccountDetailPage() {
  const { id } = Route.useParams();
  return <div>Detail for {id}</div>;
}
