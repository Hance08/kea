export function Placeholder({ id }: { id: string }) {
  return (
    <div className="flex h-full items-center justify-center text-sm text-muted-foreground">
      {id} — coming soon
    </div>
  );
}
