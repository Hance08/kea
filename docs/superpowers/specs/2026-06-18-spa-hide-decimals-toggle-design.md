# SPA hide-decimals UI toggle — Design

**Date:** 2026-06-18
**Status:** Approved

## Goal

Let users flip `display.hide_decimals` from the SPA without editing
`~/Library/Application Support/kea/config.yaml` (or `~/.config/kea/config.yaml`
on Linux) and restarting the server.

## Current state

- `cfg.Display.HideDecimals` (bool, default false) loads from YAML config.
- `GET /api/config` returns it as `{"display":{"hide_decimals":<bool>}}`.
- The SPA reads it via `useServerConfig()` and exposes config-bound formatters
  through `useAmountFormat()` in `spa/src/lib/server-config.tsx`.
- Every SPA component and route uses the hook — no direct imports of
  `formatCents` / `formatBalanceAbs` from `@/lib/format` outside the hook
  itself and its tests.

## Out of scope

- Persisting per-user preferences (the YAML config is global per server).
- Hot-reloading other config fields (`defaults.currency`, server settings)
  — those still require a server restart.
- A "preview" mode that toggles only for the current browser session
  without writing to disk.

## Architecture

Two changes, one new HTTP endpoint and one new SPA route:

```
SPA /settings (toggle)  --PATCH /api/config-->  handlePatchConfig
       ^                                              |
       |                                              v
       |                                       viper.Set + WriteConfig
       |                                       mutate *config.Config in place
       |                                              |
       +------- invalidate ['server-config'] <--------+
                       |
                       v
              useAmountFormat re-renders
              all amounts across all pages
```

## Component 1: `PATCH /api/config`

### Route

Add to `internal/api/router.go` alongside the existing GET:

```go
r.Method(http.MethodPatch, "/config", apiHandler(s.handlePatchConfig))
```

### Handler

Lives in `internal/api/config.go`:

```go
type configPatchRequest struct {
    Display *configDisplayPatch `json:"display"`
}

type configDisplayPatch struct {
    HideDecimals *bool `json:"hide_decimals"`
}

func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) error {
    var req configPatchRequest
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(&req); err != nil {
        return badRequest("invalid JSON body: " + err.Error())
    }
    if req.Display == nil || req.Display.HideDecimals == nil {
        return badRequest("display.hide_decimals is required")
    }

    cfg := s.svc.Config()
    cfg.Display.HideDecimals = *req.Display.HideDecimals
    viper.Set("display.hide_decimals", *req.Display.HideDecimals)
    if err := viper.WriteConfig(); err != nil {
        return fmt.Errorf("failed to persist config: %w", err)
    }
    return writeJSON(w, http.StatusOK, configResponse{
        Defaults: configDefaults{Currency: cfg.Defaults.Currency},
        Display:  configDisplay{HideDecimals: cfg.Display.HideDecimals},
    })
}
```

Key points:

- `DisallowUnknownFields()` rejects extra top-level keys (`defaults`,
  `database`, `server`) at the JSON decode layer — this is how we enforce
  "must not touch other settings."
- Both `Display` and `HideDecimals` are pointers so we can distinguish
  "field omitted" from "explicit false." Today only `hide_decimals` is
  patchable, so missing → 400. If we ever add more fields, missing →
  "leave as-is."
- Mutating `cfg` in place is safe: `s.svc.Config()` returns the
  `*config.Config` that's shared through `NewServer` and used by every
  read handler. Subsequent `GET /api/config` calls see the new value
  without a server restart.
- The response echoes the full updated config (same shape as GET).
- `badRequest` is the existing error helper used elsewhere in the API
  package (returns a 400 with a JSON error body).

### Write strategy

Use `viper.Set` + `viper.WriteConfig`, matching the existing pattern in
`cmd/root.go:262` (`setCurrency`). The risk is that viper's merged in-memory
state includes defaults set via `viper.SetDefault` (`server.host`,
`server.port`, `server.cors_origins`) and could leak them into the
user's config file.

This is verified by a test (see "Tests" below). If the leak test fails,
the fallback is to re-marshal a `persistedConfig` struct subset to YAML and
write the file directly, but per the brainstorming decision we try viper
first.

### Tests

Table-driven in `internal/api/config_test.go`:

| Case | Body | Expected |
|------|------|----------|
| Happy path | `{"display":{"hide_decimals":true}}` | 200, file updated, in-memory cfg updated, subsequent GET returns true |
| Round-trip | PATCH true → PATCH false | GET returns false |
| Malformed JSON | `not json` | 400 |
| Unknown top-level field | `{"defaults":{"currency":"EUR"}}` | 400 (DisallowUnknownFields) |
| Empty body | `{}` | 400 (HideDecimals missing) |
| Empty display | `{"display":{}}` | 400 (HideDecimals missing) |
| Wrong type | `{"display":{"hide_decimals":"yes"}}` | 400 |
| Leak prevention | Load minimal YAML (only `defaults.currency: TWD` + `display.hide_decimals: true`), PATCH to false, read file back | File does NOT contain `server.host`, `server.port`, `server.cors_origins`, `database.path` |

## Component 2: SPA settings page

### New UI primitive

`spa/src/components/ui/switch.tsx` — shadcn-style switch wrapping
`@radix-ui/react-switch`. ~25 lines. Requires adding
`@radix-ui/react-switch` to `spa/package.json`. Matches the styling
conventions of the existing `label.tsx`, `input.tsx`, etc.

### API client

Add to `spa/src/lib/api.ts`:

```ts
export interface ConfigPatch {
  display?: { hide_decimals?: boolean };
}

export async function setConfig(patch: ConfigPatch): Promise<ServerConfig> {
  const res = await fetch('/api/config', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
  if (!res.ok) throw await apiError(res);
  return res.json();
}
```

(The snippet is illustrative; match the local `fetch` / error helper
pattern used by other write functions in `api.ts`.)

### Route

New file `spa/src/routes/settings.tsx`:

```tsx
export const Route = createFileRoute('/settings')({ component: SettingsPage });

function SettingsPage() {
  const { data: cfg } = useServerConfig();
  const qc = useQueryClient();
  const mutation = useMutation({
    mutationFn: (hide: boolean) => setConfig({ display: { hide_decimals: hide } }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['server-config'] }),
    onError: (err) => toast.error(`Failed to update setting: ${err.message}`),
  });

  return (
    <div className="max-w-xl space-y-6">
      <h1 className="text-2xl font-semibold">Settings</h1>
      <Card>
        <CardHeader><CardTitle>Display</CardTitle></CardHeader>
        <CardContent className="flex items-center justify-between">
          <div>
            <Label htmlFor="hide-decimals">Hide decimal places in amounts</Label>
            <p className="text-sm text-muted-foreground">
              When on, amounts like $12.00 show as $12 across all pages.
            </p>
          </div>
          <Switch
            id="hide-decimals"
            checked={cfg?.display.hide_decimals ?? false}
            disabled={mutation.isPending || !cfg}
            onCheckedChange={(checked) => mutation.mutate(checked)}
          />
        </CardContent>
      </Card>
    </div>
  );
}
```

Invalidate-only update strategy: on success, invalidate the
`['server-config']` query so `useServerConfig` re-fetches; amounts across
all pages re-render with the new precision without a manual reload. Toast
on failure (already wired via `spa/src/components/ui/sonner.tsx`).
`disabled` during the in-flight request prevents double-clicks.

### Sidebar update

Modify `spa/src/components/Sidebar.tsx`:

- Change the `<nav>` to `flex flex-col` and give the primary `<ul>`
  `flex-1` so the settings link sits pinned at the bottom regardless of
  viewport height.
- Add a separate `<ul>` (or divider + second list) at the bottom
  containing only the `Settings` item.
- Reuse the same active-state styling as the other items.
- No icon (plain text "Settings") to stay consistent with the existing
  sidebar items and avoid pulling in `lucide-react` solely for this.

## Component 3: Tests

### API

Covered above ("Tests" under Component 1).

### SPA — `setConfig` unit test

In the existing API-client test file (location matches project layout):

- Mock `fetch`, call `setConfig({ display: { hide_decimals: true } })`.
- Assert: PATCH method, `/api/config` URL, `Content-Type: application/json`,
  body is exactly `{"display":{"hide_decimals":true}}`.
- Mock a 400 response → assert the promise rejects with the parsed error.

### SPA — settings page integration test

New file `spa/src/routes/settings.test.tsx`:

- Render `<SettingsPage />` inside `withServerConfig` (with
  `hide_decimals: false` stub) using the existing `test-app.tsx` helpers.
- Mock `fetch` for the PATCH call → returns the updated config.
- Find the switch by accessible name "Hide decimal places in amounts",
  click it.
- Assert: `fetch` was called once with the right URL/method/body, and
  `queryClient.invalidateQueries({ queryKey: ['server-config'] })`
  happened (either by spying on the QueryClient or by asserting a
  subsequent re-render reflects the new value after the mocked GET
  resolves).

### `withServerConfig` helper update

In `spa/src/test/test-app.tsx`:

- Today it seeds the `['server-config']` query with a default. If the
  helper already takes an override arg, no change is needed. If not,
  extend it to accept an optional `Partial<ServerConfig>` that is
  deep-merged into the default before being written into the cache.
- Inspect the file before changing to keep the surface minimal.

### No e2e or smoke tests

The existing `useAmountFormat` consumers already prove the hook re-renders
when config changes. The toggle just changes the config; propagation is
already covered.

## Commit structure

Two commits, conventional-commits style, no `Co-Authored-By`:

1. `feat(api): add PATCH /api/config for display settings`
2. `feat(spa): add /settings route with hide_decimals toggle`

## Project conventions to honor

- SPDX header on new Go files (`internal/api/config.go` already has one
  — the new types/handler go in the same file, so no new file is needed
  for the API change).
- No SPDX on TS files.
- English only.
- All repository methods take `context.Context` first (not relevant here
  — no repo calls).
- Amounts are int64 cents (not relevant here).
