# Sagano Sniper v2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the sagano-sniper booking flow on top of the real Triplabo / Firebase auth model discovered during recon — replacing the v1 DOM-form-fill approach with direct API calls to `ars-backend.triplabo.jp`, using Playwright only to bootstrap Google login and to display the final GMO payment page.

**Architecture:** One Playwright Chromium page provides Firebase auth context (cookies + live JWT in localStorage) for all subsequent API calls. The orchestrator polls Sagano's seat-map API, holds matching seats via `/v1/reservations/check`, creates a booking session via `/v2/booking/session/put`, commits via `/v2/booking/create`, then navigates the same browser to the returned GMO payment URL and pauses for the user.

**Tech Stack:** Python 3.11+, Playwright (Chromium, headed), Pydantic v2, pytest + pytest-asyncio.

**Project root:** `/Users/hance/programming/web/sagano-sniper/` (all paths below are relative to this root). Scaffolding from v1 (`pyproject.toml`, venv, picker module, recon stubs) is already in place — this plan modifies the existing repo in place.

**Spec:** [`/Users/hance/programming/kea/docs/superpowers/specs/2026-06-12-sagano-sniper-v2-design.md`](../specs/2026-06-12-sagano-sniper-v2-design.md)

---

## Task 1: Replace recon constants with real values

**Files:**
- Modify: `recon/findings.py`
- Modify: `recon/findings.md`

- [ ] **Step 1: Rewrite `recon/findings.py`**

```python
"""Constants confirmed from a manual recon pass (Chrome Network HAR, 2026-06-12).

Source: payment.linktivity.io_Archive [26-06-12 14-43-00].har

The booking flow has TWO backends:
1. common-api.sagano.linktivity.io — seat inventory and soft hold (no auth header)
2. ars-backend.triplabo.jp        — booking session + commit (Firebase JWT required)
"""

# ── Sagano endpoints (no auth) ─────────────────────────────────────────────
SEAT_MAP_URL_TEMPLATE = (
    "https://common-api.sagano.linktivity.io/v1/inventories/"
    "{service_day}/services/{service_id}"
    "?product_id={product_id}&base_booking_id="
)

CHECK_URL = "https://common-api.sagano.linktivity.io/v1/reservations/check"

# Headers Sagano expects on /check and the seat-map fetch.
SAGANO_HEADERS = {
    "Origin": "https://file.sagano.linktivity.io",
    "Referer": "https://file.sagano.linktivity.io/",
    "Content-Type": "application/json",
}

SEAT_PICKER_URL_TEMPLATE = (
    "https://file.sagano.linktivity.io/seat/{product_id}/{direction}"
    "?lang={lang}&date={service_day}&unitsCount={units}"
    "&backUrl=&redirectUrl=&currentStep=confirm"
)

# ── Triplabo endpoints (Firebase JWT required as grpc-metadata-authorization) ──
SESSION_PUT_URL    = "https://ars-backend.triplabo.jp/v2/booking/session/put"
SESSION_CHECK_URL  = "https://ars-backend.triplabo.jp/v2/booking/session/check"
BOOKING_CREATE_URL = "https://ars-backend.triplabo.jp/v2/booking/create"
USER_GET_URL       = "https://ars-backend.triplabo.jp/v2/user/get"

# The activity page on the merchant SPA — used to boot Firebase and refresh
# the ID token. Pattern: /activity/{lang}/{activity_id}
ACTIVITY_URL_TEMPLATE = (
    "https://ars-saganokanko.triplabo.jp/activity/{lang}/{activity_id}"
)

# ── Firebase auth (Google Identity Toolkit) ────────────────────────────────
# The localStorage key the SPA uses is "firebase:authUser:<api_key>:[DEFAULT]"
# where <api_key> is the Google API key embedded in the SPA bundle.
FIREBASE_API_KEY = "AIzaSyB7lJQdGfogdQP5e_LeowXTsfdCKQnnqZk"
FIREBASE_AUTH_STORAGE_KEY = (
    f"firebase:authUser:{FIREBASE_API_KEY}:[DEFAULT]"
)

# Holds at /check expire after this window. Measured from the HAR session.
HOLD_WINDOW_SECONDS = 300
```

- [ ] **Step 2: Rewrite `recon/findings.md`**

```markdown
# Recon findings (confirmed 2026-06-12)

Source: `payment.linktivity.io_Archive [26-06-12 14-43-00].har`

## Two backends

| Domain | Purpose | Auth |
|---|---|---|
| `common-api.sagano.linktivity.io` | Seat inventory + soft seat hold | Origin/Referer headers only |
| `ars-backend.triplabo.jp` | Booking session + commit | Firebase ID token in `grpc-metadata-authorization` |

## Endpoints

- `GET  /v1/inventories/{date}/services/{service_id}?product_id={pid}&base_booking_id=` → seat map
- `POST /v1/reservations/check` → soft hold (returns `{}`)
- `POST /v2/booking/session/put` → create/update booking session
- `POST /v2/booking/session/check` → validate session
- `POST /v2/booking/create` → commit; returns `{bookingId, gmoPaymentUrl}`
- `POST /v2/user/get` → readiness probe / pulls saved profile

## Seat-map response shape

```
{
  "summaries": [...],
  "service_state": {...},
  "car_inventories": [
    {
      "logical_car_id": "1",
      "physical_car_id": "...",
      "physical_car_name": "...",
      "standing": ...,
      "arrangements": [
        {
          "inventory_id": "6209446",
          "seat_group_id": "01",         # string, may have leading zeros
          "seat_id": "A",
          "arrangement_type_id": "1",    # "3" is the premium tier
          "arrangement_state": "ARRANGEABLE" | "UNARRANGEABLE",
          "reservation_state": "VACANT" | "CONFIRMED",
          ...
        }
      ]
    }
  ]
}
```

A seat is available iff `arrangement_state == "ARRANGEABLE" AND reservation_state == "VACANT"`.

## /check request body (all values stringified)

```
{
  "product_id": "51",
  "from_station_id": "1",
  "service": {
    "service_day": "2026-07-07",
    "service_id": "44",
    "manual": {
      "seats": [{
        "logical_car_id": "4",
        "seat_group_id": "10",
        "seat_id": "B",
        "arrangement_type_id": "3"
      }]
    },
    "mutation_id": "<UUID>"
  }
}
```

## /v2/booking/session/put — first call (no session id, creates session)

```
{
  "session": {
    "languageCode": "zt",
    "activityId": "LINKTIVITY-YRBTL",
    "planId": "SAGANO-YRBTL-1",
    "targetDate": "2026-07-07",
    "planStartTimeId": "LINKTIVITY-YRBTL-1-1",
    "planStartTime": "",
    "planUnitItems": [
      {"id": "CAESBAoCCAw=", "title": "成人 (12歳以上)", "count": 2},
      {"id": "CAMSCAoCCAYSAggL", "title": "兒童 (6-11歳)", "count": 0}
    ],
    "currencyCode": "JPY"
  }
}
```
→ Response: `{"sessionId": "sess_..."}`

## /v2/booking/session/put — follow-up with full passenger data

Same envelope but adds `id` (the sessionId), `participantLastName`, `participantFirstName`,
`participantEmailAddress`, `destinationEmail`, `participantResidentRegion`,
`bookingNotes` (URL-encoded Sagano params), and empty arrays for fields the SPA doesn't use:

```
"perBookingFields": [],
"perParticipantsBookingFields": [{"unitId": "<unit_id>", "responses": []}, ...],
"extendedBookingFields": {},
"extendedParticipantFields": [],
"bookingNotes": "from_station_id=1&to_station_id=4&service_id=44&seats=4-10-B-3,4-10-A-3&activity_i_d=LINKTIVITY-YRBTL&lang=zt"
```

## /v2/booking/create

```
{"sessionId": "sess_...", "payments": [{"method": "CREDITCARD", "gateway": "GMO"}]}
```
→ Response includes `gmoPaymentUrl` (signed JWT) — opens the GMO payment page.

## Firebase auth

- Sign-in flow: Google OAuth via `linktivity-gds-platform` Firebase project, tenant `ars-prod-saganokanko-jlauf`.
- ID token (JWT) is stored in browser `localStorage` under
  `firebase:authUser:AIzaSyB7lJQdGfogdQP5e_LeowXTsfdCKQnnqZk:[DEFAULT]`.
- Lifetime: ~1 hour. The Firebase SDK auto-refreshes on page load.
- Sent on every `ars-backend.triplabo.jp` call as `grpc-metadata-authorization: <token>`.

## Hold window

Measured: ~5 minutes between `/check` 200 and seats being released. Stored as
`HOLD_WINDOW_SECONDS = 300` for reference; flow doesn't rely on this — the user
finishes manually inside the window.
```

- [ ] **Step 3: Verify imports still work**

```bash
cd /Users/hance/programming/web/sagano-sniper
source .venv/bin/activate
python -c "from sagano_sniper.recon import CHECK_URL, SAGANO_HEADERS, SESSION_PUT_URL, FIREBASE_AUTH_STORAGE_KEY; print('ok')"
```
Expected: `ok`.

Note: `sagano_sniper/recon.py` (the re-export module) needs new symbols. Update it in Step 4.

- [ ] **Step 4: Update `sagano_sniper/recon.py` re-exports**

Replace the file with:

```python
"""Re-exports recon constants for import by the rest of the package."""
from recon.findings import (
    ACTIVITY_URL_TEMPLATE,
    BOOKING_CREATE_URL,
    CHECK_URL,
    FIREBASE_API_KEY,
    FIREBASE_AUTH_STORAGE_KEY,
    HOLD_WINDOW_SECONDS,
    SAGANO_HEADERS,
    SEAT_MAP_URL_TEMPLATE,
    SEAT_PICKER_URL_TEMPLATE,
    SESSION_CHECK_URL,
    SESSION_PUT_URL,
    USER_GET_URL,
)

__all__ = [
    "ACTIVITY_URL_TEMPLATE",
    "BOOKING_CREATE_URL",
    "CHECK_URL",
    "FIREBASE_API_KEY",
    "FIREBASE_AUTH_STORAGE_KEY",
    "HOLD_WINDOW_SECONDS",
    "SAGANO_HEADERS",
    "SEAT_MAP_URL_TEMPLATE",
    "SEAT_PICKER_URL_TEMPLATE",
    "SESSION_CHECK_URL",
    "SESSION_PUT_URL",
    "USER_GET_URL",
]
```

- [ ] **Step 5: Re-run the import sanity check**

```bash
python -c "from sagano_sniper.recon import CHECK_URL, SAGANO_HEADERS, SESSION_PUT_URL, FIREBASE_AUTH_STORAGE_KEY; print('ok')"
```
Expected: `ok`.

- [ ] **Step 6: Commit**

```bash
git add recon/ sagano_sniper/recon.py
git commit -m "feat: replace recon stubs with real Sagano + Triplabo endpoints"
```

---

## Task 2: Migrate Seat data model to string seat_group_id

**Files:**
- Modify: `sagano_sniper/seats.py`
- Modify: `tests/test_seats_models.py`

The v1 `Seat` used `seat_group_id: int`. The API returns strings (`"01"`, `"10"`).
We keep them as strings to preserve leading zeros.

- [ ] **Step 1: Rewrite `tests/test_seats_models.py`**

```python
from sagano_sniper.seats import Seat, SeatMap


def test_seat_id_string_matches_redirect_format():
    seat = Seat(
        logical_car_id=4,
        seat_group_id="10",
        seat_id="A",
        arrangement_type_id="3",
        available=True,
    )
    assert seat.id_string() == "4-10-A-3"


def test_seat_id_string_strips_leading_zero_from_group():
    """The redirect URL format uses bare integers for the group, no leading zero."""
    seat = Seat(
        logical_car_id=1,
        seat_group_id="01",
        seat_id="A",
        arrangement_type_id="1",
        available=True,
    )
    assert seat.id_string() == "1-1-A-1"


def test_seat_map_filters_available():
    seats = [
        Seat(4, "10", "A", "3", available=True),
        Seat(4, "10", "B", "3", available=False),
        Seat(4, "10", "C", "3", available=True),
    ]
    smap = SeatMap(seats=seats)
    assert [s.seat_id for s in smap.available()] == ["A", "C"]
```

- [ ] **Step 2: Run, expect 1 of 3 to fail**

```bash
source .venv/bin/activate
pytest tests/test_seats_models.py -v
```
Expected: `test_seat_id_string_strips_leading_zero_from_group` FAILS (current id_string returns "1-01-A-1"). Other two pass after import error is resolved by Step 3.

Actually, since the Seat fields have changed type (`seat_group_id` from int to str — but TypeHints aren't enforced at runtime in dataclasses), the existing tests may pass. Run them and see — the goal of this step is to expose the leading-zero bug.

- [ ] **Step 3: Update `sagano_sniper/seats.py` Seat model**

Replace the file's `Seat` and `SeatMap` block (keep `fetch_availability` for now — it'll be rewritten in Task 3):

```python
from __future__ import annotations

from dataclasses import dataclass
from typing import Any, Iterator

from sagano_sniper.recon import SEAT_MAP_URL_TEMPLATE


@dataclass(frozen=True)
class Seat:
    logical_car_id: int
    seat_group_id: str            # string from API; preserves leading zeros
    seat_id: str
    arrangement_type_id: str
    available: bool

    def id_string(self) -> str:
        """Format used in the merchant pay-page URL.

        The merchant strips leading zeros from the group, so we do too.
        """
        group_int = int(self.seat_group_id)
        return (
            f"{self.logical_car_id}-{group_int}"
            f"-{self.seat_id}-{self.arrangement_type_id}"
        )


@dataclass(frozen=True)
class SeatMap:
    seats: list[Seat]

    def available(self) -> Iterator[Seat]:
        return (s for s in self.seats if s.available)


# fetch_availability stays from Task 8 of v1 — rewritten in Task 3 of v2.
async def fetch_availability(
    page: Any,
    *,
    service_id: int,
    service_day: str,
    from_station_id: int,
    to_station_id: int,
) -> SeatMap:
    url = SEAT_MAP_URL_TEMPLATE.format(
        service_id=service_id,
        service_day=service_day,
        from_station_id=from_station_id,
        to_station_id=to_station_id,
    )
    response = await page.request.get(url)
    if not response.ok:
        raise RuntimeError(f"seat-map fetch failed: HTTP {response.status}")
    payload = await response.json()
    seats = [
        Seat(
            logical_car_id=s["logical_car_id"],
            seat_group_id=s["seat_group_id"],
            seat_id=s["seat_id"],
            arrangement_type_id=s["arrangement_type_id"],
            available=s["available"],
        )
        for s in payload["seats"]
    ]
    return SeatMap(seats=seats)
```

Note: `arrangement_type_id` also becomes `str` (was `int`).

- [ ] **Step 4: Update old fetch test fixture to use string types**

Open `tests/test_seats_fetch.py` and locate the fake response. Change every numeric `arrangement_type_id` to a string. Replace the test file's `return_value={"seats": [...]}` block where the dict values are currently `"arrangement_type_id": 3`. New values:

```python
"arrangement_type_id": "3",
```

Same for any `"seat_group_id"` if numeric. Leave the file otherwise unchanged — it'll be rewritten in Task 3.

- [ ] **Step 5: Run model tests**

```bash
pytest tests/test_seats_models.py -v
```
Expected: 3 passed.

- [ ] **Step 6: Commit**

```bash
git add sagano_sniper/seats.py tests/test_seats_models.py tests/test_seats_fetch.py
git commit -m "refactor: seat_group_id and arrangement_type_id become strings"
```

---

## Task 3: Rewrite seat-map fetcher for real schema

**Files:**
- Modify: `sagano_sniper/seats.py` (just `fetch_availability`)
- Modify: `tests/test_seats_fetch.py`
- Create: `tests/fixtures/inventory_response.json`

- [ ] **Step 1: Create sanitized fixture**

`tests/fixtures/inventory_response.json`:

```json
{
  "summaries": [],
  "service_state": {},
  "car_inventories": [
    {
      "logical_car_id": "4",
      "physical_car_id": "p4",
      "physical_car_name": "Car 4",
      "standing": false,
      "arrangements": [
        {
          "inventory_id": "1",
          "seat_group_id": "10",
          "seat_id": "A",
          "arrangement_type_id": "3",
          "arrangement_state": "ARRANGEABLE",
          "reservation_state": "VACANT",
          "occupant_booking_id": "",
          "bulk_reservation_id": "0",
          "reservation_id": "0",
          "mark": "SEAT_NAME"
        },
        {
          "inventory_id": "2",
          "seat_group_id": "10",
          "seat_id": "B",
          "arrangement_type_id": "3",
          "arrangement_state": "ARRANGEABLE",
          "reservation_state": "CONFIRMED",
          "occupant_booking_id": "bk_x",
          "bulk_reservation_id": "0",
          "reservation_id": "0",
          "mark": "SEAT_NAME"
        },
        {
          "inventory_id": "3",
          "seat_group_id": "10",
          "seat_id": "C",
          "arrangement_type_id": "3",
          "arrangement_state": "UNARRANGEABLE",
          "reservation_state": "VACANT",
          "occupant_booking_id": "",
          "bulk_reservation_id": "0",
          "reservation_id": "0",
          "mark": "SEAT_NAME"
        }
      ]
    },
    {
      "logical_car_id": "5",
      "physical_car_id": "p5",
      "physical_car_name": "Car 5",
      "standing": false,
      "arrangements": [
        {
          "inventory_id": "10",
          "seat_group_id": "02",
          "seat_id": "A",
          "arrangement_type_id": "3",
          "arrangement_state": "ARRANGEABLE",
          "reservation_state": "VACANT",
          "occupant_booking_id": "",
          "bulk_reservation_id": "0",
          "reservation_id": "0",
          "mark": "SEAT_NAME"
        }
      ]
    }
  ]
}
```

- [ ] **Step 2: Rewrite `tests/test_seats_fetch.py`**

```python
import json
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.seats import fetch_availability


FIXTURE = Path(__file__).parent / "fixtures" / "inventory_response.json"


def _payload() -> dict:
    return json.loads(FIXTURE.read_text())


def _fake_page(*, ok: bool = True, status: int = 200, payload: dict | None = None):
    fake_response = MagicMock()
    fake_response.ok = ok
    fake_response.status = status
    fake_response.json = AsyncMock(return_value=payload or _payload())

    fake_request = MagicMock()
    fake_request.get = AsyncMock(return_value=fake_response)

    fake_page = MagicMock()
    fake_page.request = fake_request
    return fake_page, fake_request


@pytest.mark.asyncio
async def test_fetch_availability_flattens_car_inventories():
    page, _ = _fake_page()
    smap = await fetch_availability(
        page,
        service_id=44,
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
    )
    assert len(smap.seats) == 4  # 3 from car 4 + 1 from car 5


@pytest.mark.asyncio
async def test_fetch_availability_marks_arrangeable_vacant_as_available():
    page, _ = _fake_page()
    smap = await fetch_availability(
        page,
        service_id=44,
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
    )
    by_id = {(s.logical_car_id, s.seat_group_id, s.seat_id): s for s in smap.seats}
    assert by_id[(4, "10", "A")].available is True   # ARRANGEABLE + VACANT
    assert by_id[(4, "10", "B")].available is False  # ARRANGEABLE + CONFIRMED
    assert by_id[(4, "10", "C")].available is False  # UNARRANGEABLE + VACANT
    assert by_id[(5, "02", "A")].available is True


@pytest.mark.asyncio
async def test_fetch_availability_preserves_seat_group_id_string():
    page, _ = _fake_page()
    smap = await fetch_availability(
        page,
        service_id=44,
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
    )
    # car 5 has group "02" — leading zero must survive
    assert any(s.seat_group_id == "02" for s in smap.seats)


@pytest.mark.asyncio
async def test_fetch_availability_uses_correct_url():
    page, req = _fake_page()
    await fetch_availability(
        page,
        service_id=44,
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
    )
    called_url = req.get.await_args.args[0]
    assert "inventories/2099-07-12/services/44" in called_url
    assert "product_id=51" in called_url


@pytest.mark.asyncio
async def test_fetch_availability_raises_on_http_error():
    page, _ = _fake_page(ok=False, status=503)
    with pytest.raises(RuntimeError, match="503"):
        await fetch_availability(
            page,
            service_id=44,
            service_day="2099-07-12",
            from_station_id=1,
            to_station_id=4,
            product_id=51,
        )
```

- [ ] **Step 3: Run, expect failures**

```bash
pytest tests/test_seats_fetch.py -v
```
Expected: failures — `fetch_availability` signature is missing `product_id`, and parser still uses flat `{"seats": [...]}`.

- [ ] **Step 4: Rewrite `fetch_availability` in `sagano_sniper/seats.py`**

Replace the existing `fetch_availability` function with:

```python
async def fetch_availability(
    page: Any,
    *,
    service_id: int,
    service_day: str,
    from_station_id: int,
    to_station_id: int,
    product_id: int,
) -> SeatMap:
    """GET the seat-map for a given service/date and flatten to a SeatMap.

    Note: from_station_id and to_station_id are accepted for API symmetry
    with the rest of the codebase but are not used in the URL — the inventory
    endpoint is keyed only by service_day + service_id + product_id.
    """
    url = SEAT_MAP_URL_TEMPLATE.format(
        service_day=service_day,
        service_id=service_id,
        product_id=product_id,
    )
    response = await page.request.get(url)
    if not response.ok:
        raise RuntimeError(f"seat-map fetch failed: HTTP {response.status}")
    payload = await response.json()

    seats: list[Seat] = []
    for car in payload.get("car_inventories", []):
        car_id = int(car["logical_car_id"])
        for arr in car.get("arrangements", []):
            seats.append(
                Seat(
                    logical_car_id=car_id,
                    seat_group_id=arr["seat_group_id"],
                    seat_id=arr["seat_id"],
                    arrangement_type_id=arr["arrangement_type_id"],
                    available=(
                        arr.get("arrangement_state") == "ARRANGEABLE"
                        and arr.get("reservation_state") == "VACANT"
                    ),
                )
            )
    return SeatMap(seats=seats)
```

- [ ] **Step 5: Run tests**

```bash
pytest tests/test_seats_fetch.py -v
```
Expected: 5 passed.

- [ ] **Step 6: Commit**

```bash
git add sagano_sniper/seats.py tests/test_seats_fetch.py tests/fixtures/inventory_response.json
git commit -m "feat: fetch_availability uses inventories endpoint with real schema"
```

---

## Task 4: Extend config models for v2

**Files:**
- Modify: `sagano_sniper/config.py`
- Modify: `tests/test_config_models.py`
- Modify: `tests/fixtures/config.toml`
- Modify: `tests/fixtures/prefs.toml`
- Modify: `tests/test_config_loader.py`

- [ ] **Step 1: Rewrite `tests/test_config_models.py`**

```python
import pytest
from pydantic import ValidationError

from sagano_sniper.config import Passenger, PlanUnit, Preference, RunConfig


def test_passenger_requires_name_email_region():
    p = Passenger(
        last_name="Chin",
        first_name="YUHAO",
        email="a@b.com",
        resident_region="OVERSEA",
    )
    assert p.last_name == "Chin"


def test_passenger_rejects_bad_email():
    with pytest.raises(ValidationError):
        Passenger(
            last_name="Chin",
            first_name="YUHAO",
            email="not-an-email",
            resident_region="OVERSEA",
        )


def test_plan_unit_requires_id_title_count():
    u = PlanUnit(unit_id="CAESBAoCCAw=", title="成人", count=2)
    assert u.count == 2


def test_preference_range_and_parity_defaults():
    pref = Preference(
        logical_car_id=5,
        seat_group_id_min=2,
        seat_group_id_max=14,
        seat_ids=["A", "B"],
    )
    assert pref.seat_group_id_parity == "any"
    assert pref.arrangement_type_id is None


def test_preference_rejects_invalid_parity():
    with pytest.raises(ValidationError):
        Preference(
            logical_car_id=5,
            seat_group_id_min=2,
            seat_group_id_max=14,
            seat_group_id_parity="sometimes",
            seat_ids=["A", "B"],
        )


def _ok_kwargs(**overrides):
    base = dict(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        activity_id="LINKTIVITY-YRBTL",
        plan_id="SAGANO-YRBTL-1",
        plan_start_time_id="LINKTIVITY-YRBTL-1-1",
        lang="zt",
        plan_units=[PlanUnit(unit_id="CAESBAoCCAw=", title="成人", count=2)],
        preferences=[Preference(
            logical_car_id=5,
            seat_group_id_min=2,
            seat_group_id_max=14,
            seat_group_id_parity="even",
            seat_ids=["A", "B"],
            arrangement_type_id="3",
        )],
        fallback_any=False,
        units=2,
        timeout_minutes=10,
    )
    base.update(overrides)
    return base


def test_run_config_accepts_future_date():
    cfg = RunConfig(**_ok_kwargs())
    assert cfg.units == 2
    assert cfg.preferences[0].seat_group_id_parity == "even"


def test_run_config_rejects_past_date():
    with pytest.raises(ValidationError):
        RunConfig(**_ok_kwargs(service_day="2020-01-01"))


def test_run_config_currency_and_payment_defaults():
    cfg = RunConfig(**_ok_kwargs())
    assert cfg.currency_code == "JPY"
    assert cfg.payment_method == "CREDITCARD"
    assert cfg.payment_gateway == "GMO"
```

- [ ] **Step 2: Run, expect import / validation failures**

```bash
pytest tests/test_config_models.py -v
```
Expected: failures — `PlanUnit` doesn't exist, Preference has wrong fields, RunConfig missing new fields.

- [ ] **Step 3: Rewrite `sagano_sniper/config.py` model section**

Open the file. Replace everything from the top through the last model class (keep the loader functions `load_config` and `build_run_config` at the bottom for now — they'll be updated in Step 5).

New top of file:

```python
from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, EmailStr, Field, field_validator


class Passenger(BaseModel):
    last_name: str = Field(min_length=1)
    first_name: str = Field(min_length=1)
    email: EmailStr
    resident_region: str = Field(min_length=1)


class PlanUnit(BaseModel):
    unit_id: str = Field(min_length=1)
    title: str = Field(min_length=1)
    count: int = Field(ge=0)


class Preference(BaseModel):
    logical_car_id: int
    seat_group_id_min: int = Field(ge=0)
    seat_group_id_max: int = Field(ge=0)
    seat_group_id_parity: Literal["any", "even", "odd"] = "any"
    seat_ids: list[str] = Field(min_length=1)
    arrangement_type_id: str | None = None


class RunConfig(BaseModel):
    service_day: str
    from_station_id: int
    to_station_id: int
    product_id: int
    service_id: int
    direction: Literal["up", "down"]
    activity_id: str
    plan_id: str
    plan_start_time_id: str
    currency_code: str = "JPY"
    payment_method: str = "CREDITCARD"
    payment_gateway: str = "GMO"
    lang: str
    plan_units: list[PlanUnit] = Field(min_length=1)
    preferences: list[Preference] = Field(min_length=1)
    fallback_any: bool
    units: int = Field(ge=1)
    timeout_minutes: int = Field(ge=1)

    @field_validator("service_day")
    @classmethod
    def must_be_future(cls, v: str) -> str:
        d = date.fromisoformat(v)
        if d < date.today():
            raise ValueError("service_day must be today or in the future")
        return v
```

- [ ] **Step 4: Run model tests**

```bash
pytest tests/test_config_models.py -v
```
Expected: 9 passed.

- [ ] **Step 5: Update loader fixtures**

`tests/fixtures/config.toml`:

```toml
[passenger]
last_name = "Tanaka"
first_name = "Alice"
email = "alice@example.com"
resident_region = "OVERSEA"
```

`tests/fixtures/prefs.toml`:

```toml
fallback_any = false

[[plan_units]]
unit_id = "CAESBAoCCAw="
title = "成人 (12歳以上)"
count = 2

[[preferences]]
logical_car_id = 5
seat_group_id_min = 2
seat_group_id_max = 14
seat_group_id_parity = "even"
seat_ids = ["A", "B"]
arrangement_type_id = "3"

[[preferences]]
logical_car_id = 5
seat_group_id_min = 2
seat_group_id_max = 14
seat_group_id_parity = "even"
seat_ids = ["C", "D"]
arrangement_type_id = "3"
```

- [ ] **Step 6: Update `build_run_config` in `sagano_sniper/config.py`**

Find `build_run_config` near the bottom of the file. Replace it with:

```python
def build_run_config(
    *,
    service_day: str,
    from_station_id: int,
    to_station_id: int,
    product_id: int,
    service_id: int,
    direction: str,
    activity_id: str,
    plan_id: str,
    plan_start_time_id: str,
    lang: str,
    prefs_path: Path,
    timeout_minutes: int,
    units: int,
) -> RunConfig:
    prefs_data = tomllib.loads(Path(prefs_path).read_text())
    return RunConfig(
        service_day=service_day,
        from_station_id=from_station_id,
        to_station_id=to_station_id,
        product_id=product_id,
        service_id=service_id,
        direction=direction,
        activity_id=activity_id,
        plan_id=plan_id,
        plan_start_time_id=plan_start_time_id,
        lang=lang,
        plan_units=[PlanUnit(**u) for u in prefs_data["plan_units"]],
        preferences=[Preference(**p) for p in prefs_data["preferences"]],
        fallback_any=prefs_data.get("fallback_any", False),
        timeout_minutes=timeout_minutes,
        units=units,
    )
```

The `load_config` function (generic Pydantic loader) stays unchanged.

- [ ] **Step 7: Update `tests/test_config_loader.py`**

Replace the body so the test_build_run_config_from_args_and_prefs_file uses the new signature:

```python
from pathlib import Path

from sagano_sniper.config import Passenger, load_config

FIXTURES = Path(__file__).parent / "fixtures"


def test_load_passenger():
    p = load_config(FIXTURES / "config.toml", "passenger", Passenger)
    assert p.last_name == "Tanaka"
    assert p.email == "alice@example.com"


def test_build_run_config_from_args_and_prefs_file():
    from sagano_sniper.config import build_run_config

    cfg = build_run_config(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        activity_id="LINKTIVITY-YRBTL",
        plan_id="SAGANO-YRBTL-1",
        plan_start_time_id="LINKTIVITY-YRBTL-1-1",
        lang="zt",
        prefs_path=FIXTURES / "prefs.toml",
        timeout_minutes=10,
        units=2,
    )
    assert cfg.fallback_any is False
    assert len(cfg.preferences) == 2
    assert cfg.preferences[0].seat_group_id_parity == "even"
    assert cfg.plan_units[0].count == 2
```

- [ ] **Step 8: Run loader tests**

```bash
pytest tests/test_config_models.py tests/test_config_loader.py -v
```
Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add sagano_sniper/config.py tests/test_config_models.py tests/test_config_loader.py tests/fixtures/
git commit -m "feat: extend config models for Triplabo session payload"
```

---

## Task 5: Rewrite picker with range + parity matching

**Files:**
- Modify: `sagano_sniper/picker.py`
- Modify: `tests/test_picker.py`

- [ ] **Step 1: Rewrite `tests/test_picker.py`**

```python
from sagano_sniper.config import Preference
from sagano_sniper.picker import pick
from sagano_sniper.seats import Seat, SeatMap


def _smap(*seats: Seat) -> SeatMap:
    return SeatMap(seats=list(seats))


def _pref(**kw) -> Preference:
    base = dict(
        logical_car_id=5,
        seat_group_id_min=2,
        seat_group_id_max=14,
        seat_group_id_parity="even",
        seat_ids=["A", "B"],
        arrangement_type_id="3",
    )
    base.update(kw)
    return Preference(**base)


def test_pick_returns_paired_seats_from_first_even_group_where_both_free():
    smap = _smap(
        Seat(5, "02", "A", "3", available=True),
        Seat(5, "02", "B", "3", available=False),    # B taken in group 02
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),     # group 04 has both
        Seat(5, "06", "A", "3", available=True),
        Seat(5, "06", "B", "3", available=True),
    )
    result = pick(smap, [_pref()], units=2, fallback_any=False)
    assert result is not None
    assert [(s.seat_group_id, s.seat_id) for s in result] == [("04", "A"), ("04", "B")]


def test_pick_skips_odd_groups_when_parity_even():
    smap = _smap(
        Seat(5, "03", "A", "3", available=True),
        Seat(5, "03", "B", "3", available=True),    # odd group ignored
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    )
    result = pick(smap, [_pref()], units=2, fallback_any=False)
    assert result is not None
    assert int(result[0].seat_group_id) == 4


def test_pick_respects_arrangement_type_filter():
    smap = _smap(
        Seat(5, "02", "A", "1", available=True),    # wrong arrangement type
        Seat(5, "02", "B", "1", available=True),
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    )
    result = pick(smap, [_pref()], units=2, fallback_any=False)
    assert result is not None
    assert int(result[0].seat_group_id) == 4


def test_pick_returns_none_when_no_group_has_full_pair():
    smap = _smap(
        Seat(5, "02", "A", "3", available=True),
        Seat(5, "02", "B", "3", available=False),
        Seat(5, "04", "A", "3", available=False),
        Seat(5, "04", "B", "3", available=True),
    )
    result = pick(smap, [_pref()], units=2, fallback_any=False)
    assert result is None


def test_pick_falls_through_to_second_preference():
    p1 = _pref(seat_ids=["A", "B"])
    p2 = _pref(seat_ids=["C", "D"])
    smap = _smap(
        Seat(5, "04", "A", "3", available=False),
        Seat(5, "04", "B", "3", available=False),
        Seat(5, "06", "C", "3", available=True),
        Seat(5, "06", "D", "3", available=True),
    )
    result = pick(smap, [p1, p2], units=2, fallback_any=False)
    assert result is not None
    assert {s.seat_id for s in result} == {"C", "D"}


def test_pick_returns_seats_in_user_specified_order():
    smap = _smap(
        Seat(5, "04", "B", "3", available=True),
        Seat(5, "04", "A", "3", available=True),
    )
    result = pick(smap, [_pref(seat_ids=["A", "B"])], units=2, fallback_any=False)
    assert result is not None
    assert [s.seat_id for s in result] == ["A", "B"]


def test_pick_falls_back_to_any_when_no_preference_matches():
    smap = _smap(
        Seat(9, "99", "Z", "1", available=True),
        Seat(9, "99", "Y", "1", available=True),
    )
    result = pick(smap, [_pref()], units=2, fallback_any=True)
    assert result is not None
    assert {s.seat_id for s in result} == {"Z", "Y"}


def test_pick_parity_any_walks_every_group_in_range():
    pref = _pref(seat_group_id_min=2, seat_group_id_max=3, seat_group_id_parity="any")
    smap = _smap(
        Seat(5, "02", "A", "3", available=False),
        Seat(5, "02", "B", "3", available=False),
        Seat(5, "03", "A", "3", available=True),
        Seat(5, "03", "B", "3", available=True),
    )
    result = pick(smap, [pref], units=2, fallback_any=False)
    assert result is not None
    assert int(result[0].seat_group_id) == 3
```

- [ ] **Step 2: Run, expect failures**

```bash
pytest tests/test_picker.py -v
```
Expected: failures — current `pick` uses `pref.seat_group_id` (single int), not range/parity.

- [ ] **Step 3: Rewrite `sagano_sniper/picker.py`**

```python
from __future__ import annotations

from sagano_sniper.config import Preference
from sagano_sniper.seats import Seat, SeatMap


def _candidate_groups(pref: Preference) -> list[int]:
    """Expand a preference's range + parity into a list of candidate group IDs."""
    candidates = []
    for g in range(pref.seat_group_id_min, pref.seat_group_id_max + 1):
        if pref.seat_group_id_parity == "any":
            candidates.append(g)
        elif pref.seat_group_id_parity == "even" and g % 2 == 0:
            candidates.append(g)
        elif pref.seat_group_id_parity == "odd" and g % 2 == 1:
            candidates.append(g)
    return candidates


def _match_in_group(
    smap: SeatMap, pref: Preference, candidate_group: int
) -> list[Seat] | None:
    """Return seats from `pref.seat_ids` if all are available in the given group.

    Returns seats in the order specified by `pref.seat_ids`.
    """
    matching = [
        s
        for s in smap.available()
        if s.logical_car_id == pref.logical_car_id
        and int(s.seat_group_id) == candidate_group
        and (
            pref.arrangement_type_id is None
            or s.arrangement_type_id == pref.arrangement_type_id
        )
        and s.seat_id in pref.seat_ids
    ]
    available_letters = {s.seat_id for s in matching}
    if not all(sid in available_letters for sid in pref.seat_ids):
        return None
    by_letter = {s.seat_id: s for s in matching}
    return [by_letter[sid] for sid in pref.seat_ids]


def pick(
    smap: SeatMap,
    prefs: list[Preference],
    units: int,
    fallback_any: bool,
) -> list[Seat] | None:
    """Find seats matching the first preference that has a fully-available group."""
    for pref in prefs:
        for candidate in _candidate_groups(pref):
            match = _match_in_group(smap, pref, candidate)
            if match:
                return match
    if fallback_any:
        any_avail = list(smap.available())
        if len(any_avail) >= units:
            return any_avail[:units]
    return None
```

- [ ] **Step 4: Run tests**

```bash
pytest tests/test_picker.py -v
```
Expected: 8 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/picker.py tests/test_picker.py
git commit -m "feat: picker matches by car + group range/parity + seat letters"
```

---

## Task 6: Delete v1 flow + tests (clean break)

**Files:**
- Delete: `sagano_sniper/flow.py`
- Delete: `tests/test_flow_polling.py`
- Delete: `tests/test_flow_handoff.py`
- Delete: `tests/test_flow_run.py`

- [ ] **Step 1: Delete the v1 flow module and its tests**

```bash
cd /Users/hance/programming/web/sagano-sniper
rm sagano_sniper/flow.py tests/test_flow_polling.py tests/test_flow_handoff.py tests/test_flow_run.py
```

- [ ] **Step 2: Verify the rest of the suite still passes (without flow)**

```bash
source .venv/bin/activate
pytest -v
```
Expected: all remaining tests pass (picker + seats + config). Suite shrinks to ~16 tests.

If anything imports the deleted `sagano_sniper.flow`, we'll discover it now and fix it. Currently only `cli.py` does:

- [ ] **Step 3: Temporarily neuter `cli.py` so the package still imports**

`sagano_sniper/cli.py`:

```python
def main() -> None:
    raise SystemExit(
        "cli not yet wired for v2. See Task 10 of the v2 implementation plan."
    )
```

- [ ] **Step 4: Verify package import**

```bash
python -c "import sagano_sniper.cli; print('ok')"
```
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor: remove v1 flow module ahead of v2 rewrite"
```

---

## Task 7: Create auth module

**Files:**
- Create: `sagano_sniper/auth.py`
- Test: `tests/test_auth.py`

- [ ] **Step 1: Write failing tests**

`tests/test_auth.py`:

```python
import json
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.auth import (
    AuthExpired,
    get_firebase_token,
    open_with_auth,
)


@pytest.mark.asyncio
async def test_get_firebase_token_reads_localstorage_key():
    stored = {"stsTokenManager": {"accessToken": "TOKEN_XYZ"}}
    fake_page = MagicMock()
    fake_page.evaluate = AsyncMock(return_value=json.dumps(stored))
    token = await get_firebase_token(fake_page)
    assert token == "TOKEN_XYZ"
    fake_page.evaluate.assert_awaited_once()
    eval_arg = fake_page.evaluate.await_args.args[0]
    assert "firebase:authUser" in eval_arg


@pytest.mark.asyncio
async def test_get_firebase_token_raises_when_storage_missing():
    fake_page = MagicMock()
    fake_page.evaluate = AsyncMock(return_value=None)
    with pytest.raises(AuthExpired, match="no Firebase"):
        await get_firebase_token(fake_page)


@pytest.mark.asyncio
async def test_open_with_auth_restores_storage_state(tmp_path):
    auth_path = tmp_path / "auth.json"
    auth_path.write_text("{}")

    fake_page = MagicMock()
    fake_page.goto = AsyncMock()
    fake_context = MagicMock(new_page=AsyncMock(return_value=fake_page))
    fake_browser = MagicMock(
        new_context=AsyncMock(return_value=fake_context),
        close=AsyncMock(),
    )
    fake_pw = MagicMock()
    fake_pw.chromium.launch = AsyncMock(return_value=fake_browser)

    browser, context, page = await open_with_auth(fake_pw, auth_path)

    fake_pw.chromium.launch.assert_awaited_once()
    fake_browser.new_context.assert_awaited_once()
    new_context_kwargs = fake_browser.new_context.await_args.kwargs
    assert str(auth_path) in str(new_context_kwargs.get("storage_state"))
    fake_page.goto.assert_awaited()  # navigated to activity page


@pytest.mark.asyncio
async def test_open_with_auth_raises_when_storage_missing(tmp_path):
    auth_path = tmp_path / "missing.json"

    fake_pw = MagicMock()
    with pytest.raises(FileNotFoundError):
        await open_with_auth(fake_pw, auth_path)
```

- [ ] **Step 2: Run, expect failure**

```bash
pytest tests/test_auth.py -v
```
Expected: ImportError.

- [ ] **Step 3: Implement `sagano_sniper/auth.py`**

```python
"""Firebase / Google OAuth handling for sagano-sniper.

Two-step model:
1. `login(storage_path)` launches a Playwright browser headed, asks the user to
   sign in to Google via the SPA, saves the browser's storage_state to disk.
2. `open_with_auth(pw, storage_path)` restores that state and opens the SPA so
   the Firebase JS SDK refreshes its ID token. `get_firebase_token(page)` then
   extracts the live token from localStorage for use in API headers.
"""
from __future__ import annotations

import asyncio
import json
import logging
from pathlib import Path
from typing import Any

from sagano_sniper.recon import (
    ACTIVITY_URL_TEMPLATE,
    FIREBASE_AUTH_STORAGE_KEY,
    USER_GET_URL,
)

log = logging.getLogger(__name__)


class AuthExpired(RuntimeError):
    """Firebase auth state is missing or expired; user must re-run `login`."""


async def login(
    storage_path: Path,
    *,
    activity_id: str,
    lang: str = "zt",
    timeout_seconds: float = 300.0,
) -> None:
    """Open a browser, wait for the user to sign in, save storage_state."""
    from playwright.async_api import async_playwright

    storage_path = Path(storage_path)
    storage_path.parent.mkdir(parents=True, exist_ok=True)

    activity_url = ACTIVITY_URL_TEMPLATE.format(lang=lang, activity_id=activity_id)

    async with async_playwright() as pw:
        browser = await pw.chromium.launch(headless=False)
        try:
            context = await browser.new_context()
            page = await context.new_page()
            await page.goto(activity_url)

            log.info(
                "Please sign in to Google in the browser window. "
                "Waiting up to %ds for /v2/user/get to return 200…",
                int(timeout_seconds),
            )
            deadline = asyncio.get_event_loop().time() + timeout_seconds
            while asyncio.get_event_loop().time() < deadline:
                try:
                    token = await get_firebase_token(page)
                except AuthExpired:
                    await asyncio.sleep(1.0)
                    continue
                resp = await page.request.post(
                    USER_GET_URL,
                    data="{}",
                    headers={
                        "grpc-metadata-authorization": token,
                        "Content-Type": "application/json",
                    },
                )
                if resp.ok:
                    log.info("login succeeded — saving storage_state to %s", storage_path)
                    await context.storage_state(path=str(storage_path))
                    return
                await asyncio.sleep(1.0)
            raise TimeoutError("login timed out waiting for /v2/user/get to succeed")
        finally:
            await browser.close()


async def open_with_auth(
    pw: Any,
    storage_path: Path,
    *,
    activity_id: str = "LINKTIVITY-YRBTL",
    lang: str = "zt",
) -> tuple[Any, Any, Any]:
    """Restore storage_state, open the SPA so Firebase refreshes its token."""
    storage_path = Path(storage_path)
    if not storage_path.exists():
        raise FileNotFoundError(
            f"auth state not found at {storage_path}. Run `sagano-sniper login` first."
        )

    browser = await pw.chromium.launch(headless=False)
    context = await browser.new_context(storage_state=str(storage_path))
    page = await context.new_page()
    await page.goto(ACTIVITY_URL_TEMPLATE.format(lang=lang, activity_id=activity_id))
    return browser, context, page


async def get_firebase_token(page: Any) -> str:
    """Extract the live Firebase ID token from the SPA's localStorage."""
    raw = await page.evaluate(
        f"() => localStorage.getItem({FIREBASE_AUTH_STORAGE_KEY!r})"
    )
    if not raw:
        raise AuthExpired("no Firebase auth state in localStorage")
    try:
        parsed = json.loads(raw)
        token = parsed["stsTokenManager"]["accessToken"]
    except (KeyError, ValueError) as exc:
        raise AuthExpired(f"malformed Firebase storage entry: {exc}") from exc
    if not token:
        raise AuthExpired("Firebase token field empty")
    return token
```

- [ ] **Step 4: Run tests**

```bash
pytest tests/test_auth.py -v
```
Expected: 4 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/auth.py tests/test_auth.py
git commit -m "feat: add Firebase auth bootstrap and token extraction"
```

---

## Task 8: Create Triplabo API client

**Files:**
- Create: `sagano_sniper/triplabo.py`
- Test: `tests/test_triplabo.py`

- [ ] **Step 1: Write failing tests**

`tests/test_triplabo.py`:

```python
import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.auth import AuthExpired
from sagano_sniper.triplabo import (
    booking_create,
    session_check,
    session_put,
    user_get,
)


def _fake_page_with_post(*, status=200, json_body=None, text_body=""):
    resp = MagicMock()
    resp.ok = 200 <= status < 300
    resp.status = status
    resp.json = AsyncMock(return_value=json_body or {})
    resp.text = AsyncMock(return_value=text_body)
    fake_page = MagicMock()
    fake_page.request = MagicMock(post=AsyncMock(return_value=resp))
    return fake_page


@pytest.mark.asyncio
async def test_session_put_sends_auth_header_and_returns_session_id():
    page = _fake_page_with_post(
        json_body={"common": None, "sessionId": "sess_abc"}
    )
    session_id = await session_put(page, "TOK", session={"languageCode": "zt"})
    assert session_id == "sess_abc"
    call = page.request.post.await_args
    assert call.args[0].endswith("/v2/booking/session/put")
    assert call.kwargs["headers"]["grpc-metadata-authorization"] == "TOK"
    body = json.loads(call.kwargs["data"])
    assert body == {"session": {"languageCode": "zt"}}


@pytest.mark.asyncio
async def test_session_put_raises_auth_expired_on_401():
    page = _fake_page_with_post(status=401, text_body="unauthorized")
    with pytest.raises(AuthExpired):
        await session_put(page, "BAD", session={})


@pytest.mark.asyncio
async def test_session_check_returns_status_ok():
    page = _fake_page_with_post(json_body={"common": None, "status": "OK"})
    ok = await session_check(page, "TOK", "sess_abc")
    assert ok is True
    body = json.loads(page.request.post.await_args.kwargs["data"])
    assert body == {"sessionId": "sess_abc"}


@pytest.mark.asyncio
async def test_booking_create_returns_gmo_payment_url():
    page = _fake_page_with_post(
        json_body={
            "common": None,
            "bookingId": "bk_x",
            "gmoPaymentUrl": "https://payment.linktivity.io/path?access_token=x",
            "linkpayUrl": "",
        }
    )
    url = await booking_create(
        page, "TOK", session_id="sess_abc", payment_method="CREDITCARD", gateway="GMO"
    )
    assert url.startswith("https://payment.linktivity.io/")
    body = json.loads(page.request.post.await_args.kwargs["data"])
    assert body == {
        "sessionId": "sess_abc",
        "payments": [{"method": "CREDITCARD", "gateway": "GMO"}],
    }


@pytest.mark.asyncio
async def test_booking_create_raises_runtime_error_on_5xx():
    page = _fake_page_with_post(status=503, text_body="boom")
    with pytest.raises(RuntimeError, match="503"):
        await booking_create(
            page, "TOK", session_id="sess_abc",
            payment_method="CREDITCARD", gateway="GMO",
        )


@pytest.mark.asyncio
async def test_user_get_returns_user_object():
    page = _fake_page_with_post(
        json_body={"common": None, "user": {"email": "a@b.com", "lastName": "X"}}
    )
    user = await user_get(page, "TOK")
    assert user["email"] == "a@b.com"
```

- [ ] **Step 2: Run, expect ImportError**

```bash
pytest tests/test_triplabo.py -v
```
Expected: ImportError.

- [ ] **Step 3: Implement `sagano_sniper/triplabo.py`**

```python
"""Thin client for ars-backend.triplabo.jp endpoints.

All methods take an active Playwright `page` and a Firebase ID `token`.
Calls go through `page.request.post` so the browser context's cookies carry.
"""
from __future__ import annotations

import json
import logging
from typing import Any

from sagano_sniper.auth import AuthExpired
from sagano_sniper.recon import (
    BOOKING_CREATE_URL,
    SESSION_CHECK_URL,
    SESSION_PUT_URL,
    USER_GET_URL,
)

log = logging.getLogger(__name__)


def _headers(token: str) -> dict[str, str]:
    return {
        "grpc-metadata-authorization": token,
        "Content-Type": "application/json",
    }


async def _post_json(page: Any, url: str, token: str, body: dict) -> dict:
    resp = await page.request.post(
        url, data=json.dumps(body), headers=_headers(token)
    )
    if resp.status == 401:
        raise AuthExpired(f"Triplabo {url} returned 401")
    if not resp.ok:
        text = ""
        try:
            text = await resp.text()
        except Exception:
            pass
        raise RuntimeError(f"Triplabo {url} failed: HTTP {resp.status} {text}")
    return await resp.json()


async def session_put(page: Any, token: str, *, session: dict) -> str:
    """Create or update a booking session; returns the sessionId."""
    payload = {"session": session}
    data = await _post_json(page, SESSION_PUT_URL, token, payload)
    return data["sessionId"]


async def session_check(page: Any, token: str, session_id: str) -> bool:
    data = await _post_json(page, SESSION_CHECK_URL, token, {"sessionId": session_id})
    return data.get("status") == "OK"


async def booking_create(
    page: Any,
    token: str,
    *,
    session_id: str,
    payment_method: str,
    gateway: str,
) -> str:
    """Commit the booking; returns the GMO payment URL."""
    payload = {
        "sessionId": session_id,
        "payments": [{"method": payment_method, "gateway": gateway}],
    }
    data = await _post_json(page, BOOKING_CREATE_URL, token, payload)
    url = data.get("gmoPaymentUrl") or ""
    if not url:
        log.warning(
            "booking %s succeeded but gmoPaymentUrl is empty",
            data.get("bookingId", "<unknown>"),
        )
    return url


async def user_get(page: Any, token: str) -> dict:
    data = await _post_json(page, USER_GET_URL, token, {})
    return data.get("user", {})
```

- [ ] **Step 4: Run tests**

```bash
pytest tests/test_triplabo.py -v
```
Expected: 6 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/triplabo.py tests/test_triplabo.py
git commit -m "feat: add Triplabo API client for session + booking"
```

---

## Task 9: Implement v2 flow orchestrator

**Files:**
- Create: `sagano_sniper/flow.py`
- Test: `tests/test_flow_polling.py`
- Test: `tests/test_flow_check_hold.py`
- Test: `tests/test_flow_session.py`
- Test: `tests/test_flow_run.py`

This is the biggest task. We build it up in pieces: polling first, then check-hold, then session-payload builder, then the top-level `run` that ties everything together.

### Task 9a: Polling loop (with new fetch_availability signature)

- [ ] **Step 1: Write failing test**

`tests/test_flow_polling.py`:

```python
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.config import Preference
from sagano_sniper.flow import poll_until_match
from sagano_sniper.seats import Seat, SeatMap


def _pref():
    return Preference(
        logical_car_id=5,
        seat_group_id_min=2,
        seat_group_id_max=14,
        seat_group_id_parity="even",
        seat_ids=["A", "B"],
        arrangement_type_id="3",
    )


@pytest.mark.asyncio
async def test_poll_returns_seats_on_first_match():
    smap = SeatMap(seats=[
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    ])

    async def fake_fetch(page, **kwargs):
        return smap

    result = await poll_until_match(
        page=MagicMock(), fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4, "product_id": 51},
        prefs=[_pref()], units=2, fallback_any=False,
        poll_interval=0.01, timeout_seconds=1.0,
    )
    assert result is not None
    assert [s.seat_id for s in result] == ["A", "B"]


@pytest.mark.asyncio
async def test_poll_returns_none_after_timeout():
    empty = SeatMap(seats=[])

    async def fake_fetch(page, **kwargs):
        return empty

    result = await poll_until_match(
        page=MagicMock(), fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4, "product_id": 51},
        prefs=[_pref()], units=2, fallback_any=False,
        poll_interval=0.01, timeout_seconds=0.05,
    )
    assert result is None


@pytest.mark.asyncio
async def test_poll_retries_on_fetch_error_up_to_cap():
    calls = {"n": 0}

    async def fake_fetch(page, **kwargs):
        calls["n"] += 1
        if calls["n"] < 3:
            raise RuntimeError("seat-map fetch failed: HTTP 503")
        return SeatMap(seats=[
            Seat(5, "04", "A", "3", available=True),
            Seat(5, "04", "B", "3", available=True),
        ])

    result = await poll_until_match(
        page=MagicMock(), fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4, "product_id": 51},
        prefs=[_pref()], units=2, fallback_any=False,
        poll_interval=0.01, timeout_seconds=1.0,
    )
    assert result is not None
    assert calls["n"] >= 3
```

- [ ] **Step 2: Implement `sagano_sniper/flow.py` (polling only for now)**

```python
"""v2 orchestrator: HTTP-only booking driven by a single Playwright page."""
from __future__ import annotations

import asyncio
import json
import logging
import uuid
from pathlib import Path
from typing import Any, Awaitable, Callable

from sagano_sniper.config import Passenger, Preference, RunConfig
from sagano_sniper.picker import pick
from sagano_sniper.seats import Seat, SeatMap

log = logging.getLogger(__name__)

FetchFn = Callable[..., Awaitable[SeatMap]]


class CheckFailed(RuntimeError):
    """The /check call returned non-200 — seats were taken between poll and hold."""


async def poll_until_match(
    *,
    page: Any,
    fetch: FetchFn,
    fetch_kwargs: dict[str, Any],
    prefs: list[Preference],
    units: int,
    fallback_any: bool,
    poll_interval: float,
    timeout_seconds: float,
) -> list[Seat] | None:
    loop = asyncio.get_event_loop()
    deadline = loop.time() + timeout_seconds
    consecutive_errors = 0

    while loop.time() < deadline:
        try:
            smap = await fetch(page, **fetch_kwargs)
            consecutive_errors = 0
            match = pick(smap, prefs, units, fallback_any)
            if match:
                log.info("matched seats: %s", [s.id_string() for s in match])
                return match
        except RuntimeError as exc:
            consecutive_errors += 1
            log.warning("fetch failed (%d in a row): %s", consecutive_errors, exc)
            if consecutive_errors >= 5:
                raise RuntimeError(
                    "5 consecutive seat-map failures; aborting"
                ) from exc
        await asyncio.sleep(poll_interval)
    return None
```

- [ ] **Step 3: Run polling tests**

```bash
pytest tests/test_flow_polling.py -v
```
Expected: 3 passed.

### Task 9b: /check hold

- [ ] **Step 4: Write failing test**

`tests/test_flow_check_hold.py`:

```python
import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.config import (
    PlanUnit, Preference, RunConfig,
)
from sagano_sniper.flow import CheckFailed, check_hold
from sagano_sniper.seats import Seat


def _config() -> RunConfig:
    return RunConfig(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        activity_id="LINKTIVITY-YRBTL",
        plan_id="SAGANO-YRBTL-1",
        plan_start_time_id="LINKTIVITY-YRBTL-1-1",
        lang="zt",
        plan_units=[PlanUnit(unit_id="CAESBAoCCAw=", title="成人", count=2)],
        preferences=[Preference(
            logical_car_id=5,
            seat_group_id_min=2,
            seat_group_id_max=14,
            seat_group_id_parity="even",
            seat_ids=["A", "B"],
            arrangement_type_id="3",
        )],
        fallback_any=False,
        units=2,
        timeout_minutes=10,
    )


@pytest.mark.asyncio
async def test_check_hold_posts_correct_payload():
    resp = MagicMock(ok=True, status=200)
    page = MagicMock()
    page.request = MagicMock(post=AsyncMock(return_value=resp))

    seats = [
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    ]
    await check_hold(page, _config(), seats, mutation_id="MID")

    call = page.request.post.await_args
    assert call.args[0].endswith("/v1/reservations/check")
    body = json.loads(call.kwargs["data"])
    assert body["product_id"] == "51"
    assert body["from_station_id"] == "1"
    svc = body["service"]
    assert svc["service_day"] == "2099-07-12"
    assert svc["service_id"] == "44"
    assert svc["mutation_id"] == "MID"
    assert svc["manual"]["seats"] == [
        {"logical_car_id": "5", "seat_group_id": "04",
         "seat_id": "A", "arrangement_type_id": "3"},
        {"logical_car_id": "5", "seat_group_id": "04",
         "seat_id": "B", "arrangement_type_id": "3"},
    ]
    # Sagano headers were sent
    assert call.kwargs["headers"]["Origin"] == "https://file.sagano.linktivity.io"


@pytest.mark.asyncio
async def test_check_hold_raises_check_failed_on_non_200():
    resp = MagicMock(ok=False, status=409)
    resp.text = AsyncMock(return_value="seats taken")
    page = MagicMock()
    page.request = MagicMock(post=AsyncMock(return_value=resp))

    with pytest.raises(CheckFailed, match="409"):
        await check_hold(page, _config(), [Seat(5, "04", "A", "3", True)], mutation_id="MID")
```

- [ ] **Step 5: Append `check_hold` to `sagano_sniper/flow.py`**

```python
from sagano_sniper.recon import CHECK_URL, SAGANO_HEADERS


def _build_check_payload(
    *,
    config: RunConfig,
    seats: list[Seat],
    mutation_id: str,
) -> dict:
    return {
        "product_id": str(config.product_id),
        "from_station_id": str(config.from_station_id),
        "service": {
            "service_day": config.service_day,
            "service_id": str(config.service_id),
            "manual": {
                "seats": [
                    {
                        "logical_car_id": str(s.logical_car_id),
                        "seat_group_id": s.seat_group_id,
                        "seat_id": s.seat_id,
                        "arrangement_type_id": s.arrangement_type_id,
                    }
                    for s in seats
                ]
            },
            "mutation_id": mutation_id,
        },
    }


async def check_hold(
    page: Any,
    config: RunConfig,
    seats: list[Seat],
    *,
    mutation_id: str,
) -> None:
    """POST /v1/reservations/check to hold the given seats. Raises CheckFailed on non-200."""
    payload = _build_check_payload(config=config, seats=seats, mutation_id=mutation_id)
    resp = await page.request.post(
        CHECK_URL,
        data=json.dumps(payload),
        headers=SAGANO_HEADERS,
    )
    if not resp.ok:
        body = ""
        try:
            body = await resp.text()
        except Exception:
            pass
        raise CheckFailed(f"/check failed: HTTP {resp.status} {body}")
```

- [ ] **Step 6: Run check-hold tests**

```bash
pytest tests/test_flow_check_hold.py -v
```
Expected: 2 passed.

### Task 9c: Session payload builder

- [ ] **Step 7: Write failing test**

`tests/test_flow_session.py`:

```python
from sagano_sniper.config import Passenger, PlanUnit, Preference, RunConfig
from sagano_sniper.flow import build_session_payload
from sagano_sniper.seats import Seat


def _config() -> RunConfig:
    return RunConfig(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        activity_id="LINKTIVITY-YRBTL",
        plan_id="SAGANO-YRBTL-1",
        plan_start_time_id="LINKTIVITY-YRBTL-1-1",
        lang="zt",
        plan_units=[
            PlanUnit(unit_id="CAESBAoCCAw=", title="成人 (12歳以上)", count=2),
        ],
        preferences=[Preference(
            logical_car_id=5,
            seat_group_id_min=2,
            seat_group_id_max=14,
            seat_ids=["A", "B"],
        )],
        fallback_any=False,
        units=2,
        timeout_minutes=10,
    )


def _passenger() -> Passenger:
    return Passenger(
        last_name="Chin",
        first_name="YUHAO",
        email="hancechin08@gmail.com",
        resident_region="OVERSEA",
    )


def test_build_session_payload_has_all_top_level_fields():
    seats = [
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    ]
    session = build_session_payload(_config(), _passenger(), seats)
    assert session["languageCode"] == "zt"
    assert session["activityId"] == "LINKTIVITY-YRBTL"
    assert session["planId"] == "SAGANO-YRBTL-1"
    assert session["targetDate"] == "2099-07-12"
    assert session["planStartTimeId"] == "LINKTIVITY-YRBTL-1-1"
    assert session["currencyCode"] == "JPY"
    assert session["participantLastName"] == "Chin"
    assert session["participantFirstName"] == "YUHAO"
    assert session["participantEmailAddress"] == "hancechin08@gmail.com"
    assert session["destinationEmail"] == "hancechin08@gmail.com"
    assert session["participantResidentRegion"] == "OVERSEA"


def test_build_session_payload_includes_plan_unit_items():
    seats = [Seat(5, "04", "A", "3", available=True)]
    session = build_session_payload(_config(), _passenger(), seats)
    assert session["planUnitItems"] == [
        {"id": "CAESBAoCCAw=", "title": "成人 (12歳以上)", "count": 2},
    ]


def test_build_session_payload_includes_booking_notes_url_encoded():
    seats = [
        Seat(5, "04", "A", "3", available=True),
        Seat(5, "04", "B", "3", available=True),
    ]
    session = build_session_payload(_config(), _passenger(), seats)
    notes = session["bookingNotes"]
    # Sagano-specific routing data is packed into bookingNotes
    assert "from_station_id=1" in notes
    assert "to_station_id=4" in notes
    assert "service_id=44" in notes
    assert "seats=5-4-A-3,5-4-B-3" in notes
    assert "activity_i_d=LINKTIVITY-YRBTL" in notes
    assert "lang=zt" in notes


def test_build_session_payload_includes_empty_extension_fields():
    seats = [Seat(5, "04", "A", "3", available=True)]
    session = build_session_payload(_config(), _passenger(), seats)
    assert session["perBookingFields"] == []
    assert session["extendedBookingFields"] == {}
    assert session["extendedParticipantFields"] == []
    # one responses entry per participant
    assert len(session["perParticipantsBookingFields"]) == 2  # adult count = 2
```

- [ ] **Step 8: Append `build_session_payload` to `sagano_sniper/flow.py`**

```python
def build_session_payload(
    config: RunConfig,
    passenger: Passenger,
    seats: list[Seat],
) -> dict:
    """Construct the /v2/booking/session/put payload from config + passenger + seats."""
    seats_param = ",".join(s.id_string() for s in seats)
    booking_notes = (
        f"from_station_id={config.from_station_id}"
        f"&to_station_id={config.to_station_id}"
        f"&service_id={config.service_id}"
        f"&seats={seats_param}"
        f"&activity_i_d={config.activity_id}"
        f"&lang={config.lang}"
    )

    # The /session/put payload requires one "responses" entry per participant.
    # We assume the primary unit is the first plan_units entry (adult tier).
    primary_unit = config.plan_units[0]
    per_participant = [
        {"unitId": primary_unit.unit_id, "responses": []}
        for _ in range(primary_unit.count)
    ]

    return {
        "languageCode": config.lang,
        "activityId": config.activity_id,
        "planId": config.plan_id,
        "targetDate": config.service_day,
        "planStartTimeId": config.plan_start_time_id,
        "planStartTime": "",
        "planUnitItems": [
            {"id": u.unit_id, "title": u.title, "count": u.count}
            for u in config.plan_units
        ],
        "currencyCode": config.currency_code,
        "participantLastName": passenger.last_name,
        "participantFirstName": passenger.first_name,
        "perBookingFields": [],
        "perParticipantsBookingFields": per_participant,
        "participantEmailAddress": passenger.email,
        "destinationEmail": passenger.email,
        "participantResidentRegion": passenger.resident_region,
        "extendedBookingFields": {},
        "extendedParticipantFields": [],
        "bookingNotes": booking_notes,
    }
```

- [ ] **Step 9: Run session-payload tests**

```bash
pytest tests/test_flow_session.py -v
```
Expected: 4 passed.

### Task 9d: Top-level `run` orchestrator

- [ ] **Step 10: Write failing test**

`tests/test_flow_run.py`:

```python
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from sagano_sniper.config import (
    Passenger, PlanUnit, Preference, RunConfig,
)
from sagano_sniper.flow import run


def _config() -> RunConfig:
    return RunConfig(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        activity_id="LINKTIVITY-YRBTL",
        plan_id="SAGANO-YRBTL-1",
        plan_start_time_id="LINKTIVITY-YRBTL-1-1",
        lang="zt",
        plan_units=[PlanUnit(unit_id="CAESBAoCCAw=", title="成人", count=2)],
        preferences=[Preference(
            logical_car_id=5,
            seat_group_id_min=4,
            seat_group_id_max=4,
            seat_group_id_parity="even",
            seat_ids=["A", "B"],
            arrangement_type_id="3",
        )],
        fallback_any=False,
        units=2,
        timeout_minutes=1,
    )


def _passenger() -> Passenger:
    return Passenger(
        last_name="Chin", first_name="YUHAO",
        email="a@b.com", resident_region="OVERSEA",
    )


def _full_inventory_response() -> dict:
    return {
        "summaries": [], "service_state": {},
        "car_inventories": [{
            "logical_car_id": "5",
            "physical_car_id": "p5",
            "physical_car_name": "Car 5",
            "standing": False,
            "arrangements": [
                {"inventory_id": "1", "seat_group_id": "04", "seat_id": "A",
                 "arrangement_type_id": "3", "arrangement_state": "ARRANGEABLE",
                 "reservation_state": "VACANT"},
                {"inventory_id": "2", "seat_group_id": "04", "seat_id": "B",
                 "arrangement_type_id": "3", "arrangement_state": "ARRANGEABLE",
                 "reservation_state": "VACANT"},
            ],
        }],
    }


def _fake_page() -> MagicMock:
    page = MagicMock()
    page.goto = AsyncMock()
    page.pause = AsyncMock()
    # evaluate returns the firebase storage JSON
    page.evaluate = AsyncMock(
        return_value='{"stsTokenManager": {"accessToken": "TOK"}}'
    )

    # Configure page.request.get -> inventory response
    inv_resp = MagicMock(ok=True, status=200)
    inv_resp.json = AsyncMock(return_value=_full_inventory_response())

    # Configure page.request.post -> sequence of responses:
    # 1. /check 200 {}
    # 2. /session/put -> sessionId
    # 3. /booking/create -> gmoPaymentUrl
    check_resp = MagicMock(ok=True, status=200)
    check_resp.json = AsyncMock(return_value={})
    session_resp = MagicMock(ok=True, status=200)
    session_resp.json = AsyncMock(return_value={"sessionId": "sess_x"})
    booking_resp = MagicMock(ok=True, status=200)
    booking_resp.json = AsyncMock(return_value={
        "bookingId": "bk_x",
        "gmoPaymentUrl": "https://payment.linktivity.io/pay?access_token=t",
    })

    page.request = MagicMock()
    page.request.get = AsyncMock(return_value=inv_resp)
    page.request.post = AsyncMock(side_effect=[check_resp, session_resp, booking_resp])
    return page


@pytest.mark.asyncio
async def test_run_end_to_end_happy_path(tmp_path):
    auth_path = tmp_path / "auth.json"
    auth_path.write_text("{}")

    page = _fake_page()
    context = MagicMock(new_page=AsyncMock(return_value=page))
    browser = MagicMock(
        new_context=AsyncMock(return_value=context),
        close=AsyncMock(),
    )
    pw = MagicMock()
    pw.chromium.launch = AsyncMock(return_value=browser)

    class _PWCtx:
        async def __aenter__(self): return pw
        async def __aexit__(self, *a): return False

    with patch("sagano_sniper.flow.async_playwright", return_value=_PWCtx()):
        await run(
            config=_config(),
            passenger=_passenger(),
            storage_path=auth_path,
            poll_interval=0.01,
        )

    # /check, /session/put, /booking/create all happened
    assert page.request.post.await_count == 3
    # Navigated to GMO payment page and paused
    last_goto = page.goto.await_args_list[-1].args[0]
    assert last_goto.startswith("https://payment.linktivity.io/")
    page.pause.assert_awaited_once()
```

- [ ] **Step 11: Append `run` to `sagano_sniper/flow.py`**

```python
from playwright.async_api import async_playwright

from sagano_sniper import auth, triplabo
from sagano_sniper.auth import AuthExpired
from sagano_sniper.recon import SEAT_PICKER_URL_TEMPLATE
from sagano_sniper.seats import fetch_availability


def _picker_url(config: RunConfig) -> str:
    return SEAT_PICKER_URL_TEMPLATE.format(
        product_id=config.product_id,
        direction=config.direction,
        lang=config.lang,
        service_day=config.service_day,
        units=config.units,
    )


async def _with_token_refresh(page: Any, token: str, make_call):
    """Run `make_call(token)`; if AuthExpired, refresh once and retry."""
    try:
        return await make_call(token)
    except AuthExpired:
        log.warning("Triplabo auth expired; refreshing token and retrying once")
        fresh = await auth.get_firebase_token(page)
        return await make_call(fresh)


async def run(
    *,
    config: RunConfig,
    passenger: Passenger,
    storage_path: Path,
    poll_interval: float = 1.0,
) -> None:
    mutation_id = str(uuid.uuid4())

    async with async_playwright() as pw:
        browser, context, page = await auth.open_with_auth(
            pw, storage_path,
            activity_id=config.activity_id, lang=config.lang,
        )
        try:
            token = await auth.get_firebase_token(page)
            await page.goto(_picker_url(config))

            for attempt in range(10):
                seats = await poll_until_match(
                    page=page,
                    fetch=fetch_availability,
                    fetch_kwargs={
                        "service_id": config.service_id,
                        "service_day": config.service_day,
                        "from_station_id": config.from_station_id,
                        "to_station_id": config.to_station_id,
                        "product_id": config.product_id,
                    },
                    prefs=config.preferences,
                    units=config.units,
                    fallback_any=config.fallback_any,
                    poll_interval=poll_interval,
                    timeout_seconds=config.timeout_minutes * 60,
                )
                if not seats:
                    log.warning("timed out without finding matching seats")
                    return

                try:
                    await check_hold(page, config, seats, mutation_id=mutation_id)
                except CheckFailed as exc:
                    log.warning(
                        "/check failed on attempt %d/10; re-entering polling: %s",
                        attempt + 1, exc,
                    )
                    continue

                session = build_session_payload(config, passenger, seats)
                session_id = await _with_token_refresh(
                    page, token,
                    lambda t: triplabo.session_put(page, t, session=session),
                )
                gmo_url = await _with_token_refresh(
                    page, token,
                    lambda t: triplabo.booking_create(
                        page, t,
                        session_id=session_id,
                        payment_method=config.payment_method,
                        gateway=config.payment_gateway,
                    ),
                )
                if gmo_url:
                    await page.goto(gmo_url)
                log.info("booking created; pausing for user to enter card details")
                await page.pause()
                return

            raise RuntimeError("/check failed 10 times in a row; aborting")
        finally:
            await browser.close()
```

- [ ] **Step 12: Run all tests**

```bash
pytest -v
```
Expected: all green (picker + seats + config + auth + triplabo + flow polling + check + session + run).

- [ ] **Step 13: Commit**

```bash
git add sagano_sniper/flow.py tests/test_flow_polling.py tests/test_flow_check_hold.py tests/test_flow_session.py tests/test_flow_run.py
git commit -m "feat: v2 orchestrator does /check + /session/put + /booking/create"
```

---

## Task 10: Rewrite CLI with login + run subcommands

**Files:**
- Modify: `sagano_sniper/cli.py`

- [ ] **Step 1: Replace `sagano_sniper/cli.py`**

```python
from __future__ import annotations

import argparse
import asyncio
import logging
from pathlib import Path

from sagano_sniper.auth import login
from sagano_sniper.config import (
    Passenger,
    build_run_config,
    load_config,
)
from sagano_sniper.flow import run


DEFAULT_AUTH_PATH = Path.home() / ".config" / "sagano-sniper" / "auth.json"


def _parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(prog="sagano-sniper")
    sub = p.add_subparsers(dest="cmd", required=True)

    # login subcommand
    l = sub.add_parser("login", help="one-time Google sign-in; saves auth state")
    l.add_argument("--auth", type=Path, default=DEFAULT_AUTH_PATH,
                   help=f"path to auth state file (default {DEFAULT_AUTH_PATH})")
    l.add_argument("--activity-id", required=True,
                   help="merchant activity ID, e.g. LINKTIVITY-YRBTL")
    l.add_argument("--lang", default="zt")

    # run subcommand
    r = sub.add_parser("run", help="run the sniper")
    r.add_argument("--config", type=Path, required=True,
                   help="TOML with [passenger] section")
    r.add_argument("--prefs", type=Path, required=True,
                   help="TOML with [[plan_units]] + [[preferences]] + fallback_any")
    r.add_argument("--auth", type=Path, default=DEFAULT_AUTH_PATH)
    r.add_argument("--date", required=True, help="service day YYYY-MM-DD")
    r.add_argument("--from-station", type=int, required=True)
    r.add_argument("--to-station", type=int, required=True)
    r.add_argument("--product", type=int, required=True)
    r.add_argument("--service-id", type=int, required=True)
    r.add_argument("--direction", choices=["up", "down"], required=True)
    r.add_argument("--activity-id", required=True)
    r.add_argument("--plan-id", required=True)
    r.add_argument("--plan-start-time-id", required=True)
    r.add_argument("--units", type=int, required=True,
                   help="total seat count (must match sum of plan_units counts)")
    r.add_argument("--lang", default="zt")
    r.add_argument("--timeout", type=int, default=10,
                   help="minutes to keep polling")
    r.add_argument("--poll-interval", type=float, default=1.0,
                   help="seconds between seat-map polls")

    return p.parse_args()


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )
    args = _parse_args()

    if args.cmd == "login":
        asyncio.run(
            login(args.auth, activity_id=args.activity_id, lang=args.lang)
        )
        return

    if args.cmd == "run":
        passenger = load_config(args.config, "passenger", Passenger)
        config = build_run_config(
            service_day=args.date,
            from_station_id=args.from_station,
            to_station_id=args.to_station,
            product_id=args.product,
            service_id=args.service_id,
            direction=args.direction,
            activity_id=args.activity_id,
            plan_id=args.plan_id,
            plan_start_time_id=args.plan_start_time_id,
            lang=args.lang,
            prefs_path=args.prefs,
            timeout_minutes=args.timeout,
            units=args.units,
        )
        asyncio.run(
            run(
                config=config,
                passenger=passenger,
                storage_path=args.auth,
                poll_interval=args.poll_interval,
            )
        )
        return
```

- [ ] **Step 2: Verify CLI help**

```bash
source .venv/bin/activate
sagano-sniper login --help
sagano-sniper run --help
```
Expected: both print help with their flags.

- [ ] **Step 3: Run full suite once more**

```bash
pytest -v
```
Expected: all green.

- [ ] **Step 4: Commit**

```bash
git add sagano_sniper/cli.py
git commit -m "feat: CLI has login + run subcommands for v2 flow"
```

---

## Task 11: Update example configs, README, gitignore

**Files:**
- Modify: `config.toml.example`
- Modify: `prefs.toml.example`
- Modify: `README.md`
- Modify: `.gitignore`

- [ ] **Step 1: Update `config.toml.example`**

```toml
[passenger]
last_name = "YourSurname"
first_name = "YourGivenName"
email = "you@example.com"
resident_region = "OVERSEA"   # or a Japan prefecture, e.g. "TOKYO"
```

- [ ] **Step 2: Update `prefs.toml.example`**

```toml
# If true and no preference matches, grab any N available seats.
fallback_any = false

# Pricing tiers — the unit_id values come from the Triplabo SPA.
# "成人 (12歳以上)" = adult, "兒童 (6-11歳)" = child.
# Look up these IDs once in the Network tab while booking manually.
[[plan_units]]
unit_id = "CAESBAoCCAw="
title = "成人 (12歳以上)"
count = 2

# Try preferences in order. For each, the script walks every group in
# [min, max] matching the parity, and grabs the first group where all
# seat_ids are simultaneously available (with the arrangement_type_id
# filter, if set). "3" is the premium open-air ("The Rich") tier.
[[preferences]]
logical_car_id = 5
seat_group_id_min = 2
seat_group_id_max = 14
seat_group_id_parity = "even"
seat_ids = ["A", "B"]
arrangement_type_id = "3"

[[preferences]]
logical_car_id = 5
seat_group_id_min = 2
seat_group_id_max = 14
seat_group_id_parity = "even"
seat_ids = ["C", "D"]
arrangement_type_id = "3"
```

- [ ] **Step 3: Rewrite `README.md`**

````markdown
# sagano-sniper v2

CLI that snipes Sagano Romantic Train seats the instant they're released,
creates the booking via the Triplabo API, and opens the GMO payment page for
you to enter card details.

## How it works

1. **Login (one-time):** opens a Playwright window, you sign in with Google,
   the browser auth state is saved to `~/.config/sagano-sniper/auth.json`.
2. **Run:** opens a Playwright window using the saved auth, polls Sagano's
   inventory API, holds matching seats via `/v1/reservations/check`, creates
   a booking session and commit via `ars-backend.triplabo.jp`, navigates the
   browser to the returned GMO payment URL, and pauses for you to pay.

## Setup

```bash
python3.11 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
playwright install chromium
```

## Configuration

```bash
cp config.toml.example config.toml      # passenger info
cp prefs.toml.example prefs.toml        # seat preferences + pricing tiers
```

Edit both. For `prefs.toml` you'll need the `unit_id` values from the SPA's
`/v2/booking/session/put` body — capture them once via Chrome DevTools while
booking manually.

## One-time login

```bash
sagano-sniper login --activity-id LINKTIVITY-YRBTL
```

A browser window opens. Sign in with your Google account. The script detects
success and closes the window. The saved auth state lasts as long as your
Google session does (typically weeks).

## Run

```bash
sagano-sniper run \
  --config config.toml \
  --prefs prefs.toml \
  --date 2026-07-12 \
  --from-station 1 \
  --to-station 4 \
  --product 51 \
  --service-id 44 \
  --direction down \
  --activity-id LINKTIVITY-YRBTL \
  --plan-id SAGANO-YRBTL-1 \
  --plan-start-time-id LINKTIVITY-YRBTL-1-1 \
  --units 2 \
  --timeout 10 \
  --poll-interval 1.0
```

The script polls until your preferred seats appear, holds them, creates the
booking, opens the GMO payment page, and pauses. Enter your card and click
Pay — same browser, same session.

## Tests

```bash
pytest -v
```

## Recon

See `recon/findings.md` and `recon/findings.py` for endpoint shapes and
session payload format. If Sagano or Triplabo change anything, re-capture a
HAR and update those two files.

## Migration from v1

This is a full architectural rewrite. The v1 `prefs.toml` format
(single-value `seat_group_id`) is not compatible — replace it with the new
range + parity shape from `prefs.toml.example`. The v1 `config.toml`
fields `name` and `phone` are gone, replaced by `last_name`, `first_name`,
`resident_region`.
````

- [ ] **Step 4: Update `.gitignore`**

Add to the existing `.gitignore`:

```
# Auth state contains live cookies + Firebase tokens; never commit
auth.json
```

(The existing `recon/manual-flow.har` and `config.toml`/`prefs.toml` entries already cover the other sensitive files.)

- [ ] **Step 5: Verify full suite once more**

```bash
pytest -v
```
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add config.toml.example prefs.toml.example README.md .gitignore
git commit -m "docs: update examples + README + gitignore for v2"
```

---

## Wrap-up

After this plan completes:

1. **Phase 1 acceptance (manual):** Run `sagano-sniper login --activity-id LINKTIVITY-YRBTL`, complete Google sign-in. Then run `sagano-sniper run …` against a future low-demand date with exact-seat prefs. The script should poll, find seats, hold, create the booking, and open the GMO payment page. **Do not actually pay** — close the window or hit Ctrl+C.

2. **Race-day operation:** Launch ~5 minutes before release time. The script polls the inventory endpoint at 1 Hz; the moment matching seats appear, it fires the whole chain in under 3 seconds and lands on the payment page.

3. **Token refresh / re-login:** If `/v2/user/get` returns 401 on startup (Google session expired), the script aborts cleanly with a message to re-run `login`.
