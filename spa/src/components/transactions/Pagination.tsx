import { Button } from '@/components/ui/button';

interface Props {
  total: number;
  limit: number;
  offset: number;
  onChange: (newOffset: number) => void;
}

export function Pagination({ total, limit, offset, onChange }: Props) {
  if (total <= limit) return null;

  const pageCount = Math.max(1, Math.ceil(total / limit));
  const currentPage = Math.floor(offset / limit) + 1;
  const canPrev = offset > 0;
  const canNext = offset + limit < total;

  return (
    <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
      <span>
        Page {currentPage} of {pageCount}
      </span>
      <div className="flex gap-2">
        <Button
          variant="outline"
          size="sm"
          disabled={!canPrev}
          onClick={() => onChange(Math.max(0, offset - limit))}
        >
          ‹ Prev
        </Button>
        <Button
          variant="outline"
          size="sm"
          disabled={!canNext}
          onClick={() => onChange(offset + limit)}
        >
          Next ›
        </Button>
      </div>
    </div>
  );
}
