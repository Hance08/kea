# Sagano Romantic Train Sniper v2 — Design

## Purpose

Snipe Sagano Romantic Train seats the instant they're released, using direct API calls against `common-api.sagano.linktivity.io` and `ars-backend.triplabo.jp` instead of driving the merchant SPA. Playwright is used only to bootstrap Google/Firebase authentication and to display the final GMO payment page for the user to enter card details.

This supersedes the v1 design (`2026-06-12-sagano-sniper-design.md`). Recon revealed that the merchant pay page is a SPA backed by `ars-backend.triplabo.jp` and gated by Firebase Authentication — facts not visible from the seat-picker side. The DOM-form-fill approach in v1 cannot work end-to-end without first establishing Firebase auth.

## Scope

**In scope**
- One-time Google sign-in via Playwright that persists `storage_state` to disk.
- Polling the Sagano seat-map API for a target date.
- Pattern-based preference matching (car, group range, parity, seat letters, arrangement type).
- Holding seats via `/v1/reservations/check`.
- Creating a booking session via `/v2/booking/session/put`.
- Committing the booking via `/v2/booking/create` and navigating Playwright to the returned GMO payment URL.
- Pausing for the user to enter card details and submit.

**Out of scope**
- Card entry, 3-D Secure, GMO confirmation.
- CAPTCHA solving on Google login (user solves manually).
- Multi-instance coordination.
- Booking multiple dates in a single run.
- Proactive Firebase token refresh (rely on the SDK's auto-refresh).

## Stack

- Python 3.11+
- Playwright (Chromium, headed for both `login` and `run`)
- Pydantic v2 for config validation
- pytest + pytest-asyncio for tests

## Architecture

```
+----------------+   +----------------------+   +----------------+
| sagano-sniper  |   | Playwright (1 page)  |   | Sagano API     |
|   login        |--→| Google OAuth → save  |   | Triplabo API   |
+----------------+   | storage_state.json   |   | Firebase Auth  |
                     +----------+-----------+   +-------+--------+
                                | persists                ^
+----------------+              v                         |
| sagano-sniper  |   +----------------------+              |
|   run          |--→| Restore storage_state|--------------|
+----------------+   | Open SPA in tab to   | extract JWT  |
                     | refresh Firebase JWT |              |
                     | ┌─ poll loop ──────┐ |              |
                     | │ /inventories     │─|--------------|
                     | │ picker.pick      │ |              |
                     | └────────┬─────────┘ |              |
                     |          v           |              |
                     | /reservations/check ─|--------------|
                     | /session/put         |--------------|
                     | /booking/create      |--------------|
                     |          │           |              |
                     |          v           |              |
                     | goto(gmoPaymentUrl)  |              |
                     | page.pause()         |              |
                     +----------------------+              |
```

**Two CLI subcommands:**
- `sagano-sniper login --auth <path>` — one-time browser login, saves auth state.
- `sagano-sniper run ...` — performs the actual sniping.

**Why only one Playwright page during `run`:** Playwright provides (1) cookies for `ars-backend.triplabo.jp`, (2) live Firebase JWT via `localStorage`, and (3) a window to navigate to the GMO payment page at the end. All API calls go through `page.request.post/get`, which carries cookies and lets us inject the `grpc-metadata-authorization` header.

## Components

```
sagano_sniper/
  __init__.py
  config.py        # CHANGED: extended models
  recon.py         # CHANGED: re-exports updated constants
  seats.py         # CHANGED: new URL + response schema
  picker.py        # CHANGED: range/parity matching
  auth.py          # NEW: storage_state + JWT extraction
  triplabo.py      # NEW: ars-backend.triplabo.jp client
  flow.py          # REWRITTEN: HTTP-only booking sequence
  cli.py           # CHANGED: login + run subcommands
```

**config.py — extended Pydantic models:**

```python
class Passenger(BaseModel):
    last_name: str           # "Chin"
    first_name: str          # "YUHAO"
    email: EmailStr
    resident_region: str     # "OVERSEA", "TOKYO", etc.

class PlanUnit(BaseModel):
    unit_id: str             # base64 id like "CAESBAoCCAw="
    title: str               # "成人 (12歳以上)"
    count: int               # 2

class Preference(BaseModel):
    logical_car_id: int                                # exact match
    seat_group_id_min: int                             # inclusive
    seat_group_id_max: int                             # inclusive
    seat_group_id_parity: Literal["any", "even", "odd"] = "any"
    seat_ids: list[str]                                # all must be free together
    arrangement_type_id: str | None = None             # optional filter

class RunConfig(BaseModel):
    service_day: str                       # "YYYY-MM-DD", must be today or future
    from_station_id: int
    to_station_id: int
    product_id: int
    service_id: int
    direction: Literal["up", "down"]
    activity_id: str                       # "LINKTIVITY-YRBTL"
    plan_id: str                           # "SAGANO-YRBTL-1"
    plan_start_time_id: str                # "LINKTIVITY-YRBTL-1-1"
    currency_code: str = "JPY"
    payment_method: str = "CREDITCARD"
    payment_gateway: str = "GMO"
    lang: str                              # "zt"
    plan_units: list[PlanUnit]
    preferences: list[Preference]
    fallback_any: bool
    units: int                             # total seat count (== sum of plan_units.count)
    timeout_minutes: int = Field(ge=1)
```

**auth.py — three public functions:**

```python
async def login(storage_path: Path) -> None:
    """Launches Playwright headed, navigates to the activity page,
    waits for user to complete Google OAuth, saves storage_state."""

async def open_with_auth(pw, storage_path: Path) -> tuple[Browser, BrowserContext, Page]:
    """Restores storage_state, opens a page on ars-saganokanko.triplabo.jp
    so the Firebase SDK refreshes its ID token, returns the trio."""

async def get_firebase_token(page: Page) -> str:
    """Extracts the current Firebase ID token via page.evaluate, reading
    from localStorage. Raises if not signed in."""
```

**triplabo.py — thin API client:**

```python
async def session_put(page, token, *, session: dict) -> str: ...     # returns sessionId
async def session_check(page, token, session_id: str) -> bool: ...
async def booking_create(page, token, *, session_id, payment_method, gateway) -> str: ...  # returns gmoPaymentUrl
async def user_get(page, token) -> dict: ...                          # readiness probe
```

All methods call `page.request.post(url, data=json.dumps(payload), headers={"grpc-metadata-authorization": token, "Content-Type": "application/json"})`. On 401, raise `AuthExpired`.

**seats.py — updated for real schema:**

- New `SEAT_MAP_URL_TEMPLATE = "https://common-api.sagano.linktivity.io/v1/inventories/{service_day}/services/{service_id}?product_id={product_id}&base_booking_id="`.
- `Seat.seat_group_id: str` (was `int`) — preserves leading zeros from API.
- `fetch_availability` flattens `car_inventories[].arrangements[]` into `list[Seat]`. `available = (arrangement_state == "ARRANGEABLE" and reservation_state == "VACANT")`.

**picker.py — pattern matching:**

```python
def pick(smap: SeatMap, prefs: list[Preference], units: int, fallback_any: bool) -> list[Seat] | None:
    for pref in prefs:
        # expand candidate groups from range + parity
        candidates = [
            g for g in range(pref.seat_group_id_min, pref.seat_group_id_max + 1)
            if pref.seat_group_id_parity == "any"
               or (pref.seat_group_id_parity == "even" and g % 2 == 0)
               or (pref.seat_group_id_parity == "odd" and g % 2 == 1)
        ]
        for candidate_group in candidates:
            matching = [
                s for s in smap.available()
                if s.logical_car_id == pref.logical_car_id
                and int(s.seat_group_id) == candidate_group
                and (pref.arrangement_type_id is None
                     or s.arrangement_type_id == pref.arrangement_type_id)
                and s.seat_id in pref.seat_ids
            ]
            if all(sid in {s.seat_id for s in matching} for sid in pref.seat_ids):
                # filter to just the requested seat letters, in user-specified order
                by_letter = {s.seat_id: s for s in matching}
                return [by_letter[sid] for sid in pref.seat_ids]
    if fallback_any:
        any_avail = list(smap.available())
        if len(any_avail) >= units:
            return any_avail[:units]
    return None
```

**flow.py — orchestrator:**

```python
async def run(config: RunConfig, passenger: Passenger, storage_path: Path,
              poll_interval: float = 1.0) -> None:
    async with async_playwright() as pw:
        browser, context, page = await auth.open_with_auth(pw, storage_path)
        try:
            token = await auth.get_firebase_token(page)

            # establish Sagano-side cookies/Origin via the seat picker page
            await page.goto(_picker_url(config))

            for attempt in range(10):
                seats = await poll_until_match(page, ...)
                if not seats:
                    return  # timeout
                try:
                    await _check_hold(page, config, seats)
                except CheckFailed:
                    continue  # seats taken between poll and hold; re-poll

                # Hold succeeded — from here we don't re-poll (seats are ours).
                # AuthExpired retries the specific failed call once with a fresh token.
                session_id = await _with_token_refresh(page, token,
                    lambda t: triplabo.session_put(page, t,
                        session=_build_session(config, passenger, seats)))
                gmo_url = await _with_token_refresh(page, token,
                    lambda t: triplabo.booking_create(page, t,
                        session_id=session_id,
                        payment_method=config.payment_method,
                        gateway=config.payment_gateway))
                await page.goto(gmo_url)
                await page.pause()
                return
            raise RuntimeError("/check failed 10 times in a row; aborting")


async def _with_token_refresh(page, token, call):
    """Run `call(token)`. On AuthExpired, refresh token once and retry. Re-raise if still expired."""
    try:
        return await call(token)
    except AuthExpired:
        fresh = await auth.get_firebase_token(page)
        return await call(fresh)
        finally:
            await browser.close()
```

## Data flow

**Login (one-time):**
```
sagano-sniper login --auth ~/.config/sagano-sniper/auth.json
        ↓
Playwright launches (headed)
        ↓
goto https://ars-saganokanko.triplabo.jp/activity/zt/<activity-id>
        ↓
SPA prompts Google sign-in; user completes OAuth
        ↓
Poll: /v2/user/get returns 200?  ──no──► wait 1s
        ↓ yes
context.storage_state(path=auth.json)   ← saves cookies + localStorage
        ↓
browser.close()
```

**Run (race day):**
```
RunConfig + Passenger + PlanUnits + storage_path
        ↓
async_playwright launch (headed)
        ↓
context = browser.new_context(storage_state=auth.json)
page = context.new_page()
goto activity page — Firebase SDK boots, refreshes token
        ↓
token = await page.evaluate(extract Firebase ID token from localStorage)
        ↓
goto seat-picker URL — establishes Sagano-side Origin/Referer cookies
        ↓
┌─ poll loop (every poll_interval s, max timeout) ─┐
│  page.request.get(/v1/inventories/{date}/services/{id}…)│
│  parse car_inventories[].arrangements[] → SeatMap        │
│  picker.pick → seats?                                    │
│  no? → sleep, loop                                       │
└──────────────┬──────────────────────────────────────────┘
               │ yes
               ▼
[retry up to 10x on CheckFailed:]
  page.request.post(/v1/reservations/check, headers={Origin, Referer, Content-Type},
                    body={product_id, from_station_id, service:{...,
                          manual:{seats:[{logical_car_id, seat_group_id, seat_id,
                                          arrangement_type_id}]}, mutation_id}})
        ↓ 200 {}
  build session payload (full Passenger + PlanUnits + Sagano notes)
        ↓
  triplabo.session_put → sessionId
        ↓
  triplabo.booking_create(sessionId, CREDITCARD, GMO) → gmoPaymentUrl
        ↓
goto(gmoPaymentUrl)
        ↓
page.pause()   ← human enters card and submits
```

**Two URL gotchas:**
1. `seat_group_id` is a string in the Sagano API (`"01"`, `"10"`) — kept as string internally, parsed to `int` only for range/parity matching, sent back to `/check` verbatim.
2. We never need to navigate to `/booking/pay` — the booking happens via direct API calls. We only need a Playwright page with a live Firebase session.

## Timing budget

- Polling cadence: 1 s (configurable).
- "Seats found" → "on GMO payment page": target < 3 s.
- Storage state is reused for the lifetime of the Google session (typically weeks).

## State

- **Persistent (disk):** `auth.json` — cookies + localStorage from a successful login. Owned by user, gitignored.
- **Per-run, in-memory:** `RunConfig`, `Passenger`, `PlanUnits`, `token`, `sessionId`, `mutation_id` UUID.
- **No caching between poll iterations.**

**Idempotency:**
- `mutation_id` UUID generated once per run, reused on `/check` retries.
- `/session/put` safe to retry (keyed by `sessionId`).
- `/booking/create` **not** idempotent — called at most once per successful `/check`.

## Error handling

| Failure | Response | Severity |
|---|---|---|
| `auth.json` doesn't exist | Abort: "no auth state — run `sagano-sniper login` first" | fatal, expected first-run |
| Firebase token can't be refreshed (Google session expired) | Detect via `/v2/user/get` on startup; abort: "auth expired — re-run `login`" | fatal |
| `firebase:authUser:...` key missing from localStorage | Same as expired — abort with re-login message | fatal |
| Seat-map non-200 during polling | Log, sleep, retry. 5 consecutive failures → abort | recoverable up to cap |
| Picker timeout without match | Log "no matching seats appeared", exit 1 | clean exit |
| `/v1/reservations/check` non-200 | Raise `CheckFailed` → orchestrator re-enters polling loop. Cap 10 attempts total. | recoverable up to cap |
| `/v2/booking/session/put` 4xx (non-401) | Abort. Likely config error (bad `plan_id` / `plan_start_time_id`). Log full body. | fatal |
| `/v2/booking/session/put` 401 | Re-extract token from localStorage, retry once. If still 401, abort with re-login message. | recoverable once |
| `/v2/booking/create` non-200 | Abort. **Never retry** — could double-book. Log full body. User must verify in Triplabo account before re-running. | fatal |
| `/v2/booking/create` succeeds but `gmoPaymentUrl` empty | Treat as warning. Booking committed. Print booking ID and pause. | warn |
| Playwright crash | Exception bubbles; cleanup in `finally`; user re-runs | fatal |
| Ctrl+C during polling | Clean abort, browser closes | clean abort |
| Ctrl+C during `/check` or `/booking/create` POST | Risk of partial commit. Log loudly. User must verify before re-running. | risky abort |

## Testing

- **`tests/test_picker.py`** — pattern matching: even/odd parity, range bounds, arrangement-type filter, partial group (A free but B not → skip), fallback through preferences, fallback-any with units count.
- **`tests/test_config.py`** — Passenger + Preference + RunConfig + PlanUnit validation.
- **`tests/test_seats_fetch.py`** — fixture from sanitized HAR response. Verifies `car_inventories[].arrangements[]` parsing, `ARRANGEABLE` + `VACANT` truthiness, `seat_group_id` stays string.
- **`tests/test_triplabo.py`** — mocks `page.request.post`, verifies URL / headers / payload shape per method. Covers 401 → AuthExpired.
- **`tests/test_auth.py`** — mocks Playwright context, verifies `storage_state` write/restore; JWT extraction via mocked `page.evaluate`.
- **`tests/test_flow_orchestration.py`** — end-to-end mock: poll → /check → /session/put → /booking/create → page.goto(gmoPaymentUrl) → page.pause(). Covers CheckFailed retry path and /booking/create no-retry policy.
- **`tests/test_flow_login_smoke.py`** — mocks Playwright, verifies `login` subcommand detects sign-in via `/v2/user/get` 200 and calls `storage_state(path=...)`.

**Manual phase-1 acceptance:** `sagano-sniper login` once, then `sagano-sniper run` against a future low-demand date with exact-seat prefs. Script should reach the GMO payment page without submitting payment.

## File layout

```
.
├── sagano_sniper/
├── tests/
│   ├── fixtures/
│   │   ├── inventory_response.json     # sanitized HAR sample
│   │   ├── config.toml
│   │   └── prefs.toml
│   └── ...
├── recon/
│   ├── findings.py
│   └── findings.md
├── config.toml.example
├── prefs.toml.example
├── pyproject.toml
└── README.md
```

## Migration from v1

- Keep: `picker.py` test fixtures (reusable), `pyproject.toml` (only deps change), `.gitignore`.
- Update in place: `config.py`, `seats.py`, `recon/findings.py`, `recon/findings.md`.
- Replace: `flow.py`, `cli.py`.
- Add: `auth.py`, `triplabo.py`.
- `Preference` model in v1 is incompatible with v2 — `prefs.toml` format changes. Note in README.

## Open questions

None — all design decisions are made. `recon/findings.py` carries the real values from the HAR.
