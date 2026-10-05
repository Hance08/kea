# Sagano Romantic Train Sniper — Design

## Purpose

Automate booking Sagano Romantic Train seats the instant they become available, racing against other buyers when popular dates open for reservation. Script grabs seats, fills passenger info, and stops before payment so the user verifies and submits manually.

## Scope

**In scope**
- Polling the Sagano seat-map API for a target date.
- Matching available seats against an ordered preference list.
- Holding seats via the `/v1/reservations/check` endpoint.
- Navigating the browser to the merchant pay page with valid session.
- Auto-filling passenger info from a config file.
- Pausing the browser for the user to verify and submit.

**Out of scope**
- Payment entry, card handling, 3-D Secure.
- CAPTCHA solving (user solves manually if encountered via `page.pause()`).
- Multi-instance coordination, distributed runs, scheduling daemon.
- Booking multiple dates in a single run.

## Stack

- **Language:** Python 3.11+
- **Browser automation:** Playwright (Chromium, headed mode)
- **Config:** TOML via `tomllib` (stdlib) + Pydantic for validation
- **Tests:** pytest, pytest-asyncio

## Phases

### Phase 0 — Recon (user-driven, ~30 min)

User walks a full manual booking flow on a non-target date with seats available, capturing artifacts:

- Open Chrome DevTools (Network tab, "Preserve log" on, "Disable cache" on).
- Walk the booking flow: seat picker → next step → passenger-info form → stop before submitting payment.
- Export as HAR file → `recon/manual-flow.har`.
- Wait on the pay page until the seat hold expires; note elapsed time.

Assistant then extracts from the HAR and writes `recon/findings.py`:

- Seat-map GET endpoint (URL template, query params, response JSON schema).
- Cookie / storage names required by the pay page (verified by deletion test).
- Redirect URL pattern for both `up` and `down` directions.
- Passenger-info form field selectors (CSS or `name` attrs).
- Measured seat-hold window after `/check`.

The script depends on these constants. If Sagano changes the site, recon must be re-run.

### Phase 1 — Build

Implement the `sagano_sniper` Python package per Components section below. Acceptance: end-to-end run against a low-demand date with `--dry-run` flag lands on a pre-filled pay page without making a real reservation.

### Phase 2 — Race-day run

User launches with target date / prefs 5 minutes before release time. Script polls, fires when seats appear, hands off browser for user to click Pay.

## Components

```
sagano_sniper/
  __init__.py
  config.py       # Pydantic models + TOML loader
  recon.py        # imports recon/findings.py constants
  seats.py        # poll seat-map, return SeatMap
  picker.py       # pure: preferences → chosen seats
  flow.py         # Playwright orchestration
  cli.py          # argparse entry
```

**config.py** — Pydantic models `Passenger`, `Preference`, `RunConfig`. Loader reads TOML config (passenger details) and merges with CLI args (route, date, units, prefs). Validates: date in future, prefs reference real cars/groups in `recon` constants, units >= 1.

**recon.py** — Thin module re-exporting constants from `recon/findings.py`: `SEAT_MAP_URL_TEMPLATE`, `CHECK_URL`, `PAY_URL_TEMPLATE`, `FORM_SELECTORS: dict[str, str]`, `HOLD_WINDOW_SECONDS`. Keeping these as typed Python (not parsed markdown) makes selector changes trackable in git.

**seats.py** — `async def fetch_availability(page, params) -> SeatMap`. Uses `page.request.get` so cookies from the loaded picker page carry. Returns `SeatMap` (list of `Seat` dataclasses with `logical_car_id`, `seat_group_id`, `seat_id`, `arrangement_type_id`, `available: bool`).

**picker.py** — Pure function `pick(available: SeatMap, prefs: list[Preference], units: int) -> list[Seat] | None`. No I/O. Tries each preference in order; if `prefs.fallback_any=True` and none match, returns any N seats. Fully unit-testable.

**flow.py** — `async def run(config: RunConfig, passenger: Passenger) -> None`. Orchestrates: launch Playwright → goto picker → poll loop → `/check` → goto pay → fill form → `page.pause()`. Takes injectable Playwright context for testing.

**cli.py** — `sagano-sniper run --config config.toml --date 2026-07-12 --from 1 --to 4 --product 51 --units 2 --prefs prefs.toml [--dry-run] [--timeout 10]`.

## Data flow

```
CLI args + config.toml
        ↓
   RunConfig + Passenger
        ↓
launch Playwright (headed) ──→ goto(seat_picker_url)
        ↓                            ↓
   wait DOMContentLoaded     picker JS runs, sets cookies/storage
        ↓
┌─── polling loop (every 1s) ───┐
│  page.request.get(seat_map)    │
│        ↓                       │
│  parse → SeatMap               │
│        ↓                       │
│  picker.pick(map, prefs)       │
│        ↓                       │
│  match? ──no──► loop again     │
└────────┬──────────────────────┘
         │ yes
         ↓
page.request.post(/v1/reservations/check, {seats, mutation_id})
         ↓
      HTTP 200 {}
         ↓
page.goto(/booking/pay?seats=...&step=input_info)
         ↓
fill form fields from Passenger via FORM_SELECTORS
         ↓
page.pause()  ← human takes over, clicks Pay
```

## Timing budget

- Polling cadence: **1 second.** Fast enough to grab seats within a few seconds of release; human-paced enough to avoid rate-limit signals.
- "Seats found" → "on pay page with form filled": target **< 3 seconds.**
- Seat hold window after `/check`: assumed ~5 minutes; actual value measured during recon and stored as `HOLD_WINDOW_SECONDS`.

## State

- **Cookies & session:** Playwright `BrowserContext`, established by loading picker page; carries automatically through `page.request.*` and `page.goto`.
- **Preferences, config:** in-memory, loaded once at startup.
- **Last-seen seat map:** in-memory only; every poll is authoritative.
- **mutation_id:** UUID generated once per run, reused on `/check` retries (idempotency).
- **No persistence** between runs.

## Error handling

| Failure | Response |
|---|---|
| Seat-map non-200 during polling | Log, sleep 2s, retry. After 5 consecutive failures, abort: "Sagano API down or rate-limited". |
| `/check` non-200 / error body | Log body, re-enter polling loop (someone beat us). Cap at 10 attempts. |
| Redirect lands on `/home` | Session invalidated. Abort loudly — recon assumption broken. |
| Form selector missing on pay page | Abort: "Selector `X` missing — re-run recon". No guessing. |
| Timeout (`--timeout`, default 10 min) without match | Exit cleanly with summary of seen seats. |
| Playwright crash | Exception bubbles; user re-launches. No auto-recovery. |
| User Ctrl+C | Close browser cleanly, log "aborted by user". |

## Testing

- **`tests/test_picker.py`** — `picker.pick` is the only branching logic. Feed fixture `SeatMap`s, assert correct seats returned for various preference lists. Covers: exact match, fallback to second preference, fallback-any, no match.
- **`tests/test_config.py`** — Config validation: bad dates, missing fields, malformed prefs.
- **`tests/test_flow_smoke.py`** — Mocked Playwright context. Asserts orchestrator calls endpoints in right order with right payloads. No real network.
- **Manual end-to-end (Phase 1 acceptance):** Real low-demand date with `--dry-run` flag that stops before `/check` POST.
- **No live-fire test of `/check` + pay flow.** Only race day, with real intent.

**Fixtures:** `tests/fixtures/seat_maps/` holds sanitized response samples captured from the recon HAR.

## File layout

```
.
├── sagano_sniper/         # package
├── tests/
│   ├── fixtures/seat_maps/
│   ├── test_picker.py
│   ├── test_config.py
│   └── test_flow_smoke.py
├── recon/
│   ├── manual-flow.har    # gitignored (may contain PII)
│   ├── findings.py        # checked in (no PII)
│   └── findings.md        # human-readable summary
├── config.toml.example
├── prefs.toml.example
├── pyproject.toml
└── README.md
```

## Open questions

None — all design decisions made. Recon will fill in concrete endpoint URLs, cookie names, and form selectors before Phase 1 begins.
