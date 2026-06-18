import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { setConfig } from '@/lib/api';
import { useServerConfig } from '@/lib/server-config';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createFileRoute } from '@tanstack/react-router';
import { toast } from 'sonner';

export const Route = createFileRoute('/settings')({ component: SettingsPage });

function SettingsPage() {
  const cfg = useServerConfig();
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (hide: boolean) => setConfig({ display: { hide_decimals: hide } }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['server-config'] });
    },
    onError: (err: Error) => {
      toast.error(err.message);
    },
  });

  return (
    <div className="max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <Card>
        <CardHeader>
          <CardTitle>Display</CardTitle>
        </CardHeader>
        <CardContent className="flex items-center justify-between gap-4">
          <div className="space-y-1">
            <Label htmlFor="hide-decimals" className="text-sm">
              Hide decimal places in amounts
            </Label>
            <p id="hide-decimals-desc" className="text-sm text-muted-foreground">
              When on, amounts like $12.00 display as $12 across all pages.
            </p>
          </div>
          <Switch
            id="hide-decimals"
            aria-describedby="hide-decimals-desc"
            checked={cfg.display.hide_decimals}
            disabled={mutation.isPending}
            onCheckedChange={(checked) => mutation.mutate(checked)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
