# Sagano Sniper Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a Python CLI that snipes Sagano Romantic Train seats the moment they're released, fills passenger info, and stops before payment.

**Architecture:** Single Python package driven by a Playwright headed browser. A polling loop watches the seat-map API; on match, the script POSTs to `/check`, navigates the same browser context to the merchant pay page, fills the passenger form, then pauses for the user to verify and submit.

**Tech Stack:** Python 3.11+, Playwright (Chromium), Pydantic v2, pytest + pytest-asyncio, TOML configs.

**Project root:** `/Users/hance/programming/web/sagano-sniper/` (all paths below are relative to this root).

**Prerequisites:** Recon (Phase 0) must be complete and `recon/findings.py` must hold real values before Phase 2 race-day run. Phase 1 build uses stub constants and mocked Playwright; recon outputs are wired in at the end.

---

## Task 1: Project scaffold

**Files:**
- Create: `pyproject.toml`
- Create: `.gitignore`
- Create: `sagano_sniper/__init__.py`
- Create: `tests/__init__.py`
- Create: `tests/conftest.py`

- [ ] **Step 1: Create project directory and init git**

```bash
mkdir -p /Users/hance/programming/web/sagano-sniper
cd /Users/hance/programming/web/sagano-sniper
git init
```

- [ ] **Step 2: Write `pyproject.toml`**

```toml
[project]
name = "sagano-sniper"
version = "0.1.0"
description = "Snipe Sagano Romantic Train reservations."
requires-python = ">=3.11"
dependencies = [
  "playwright>=1.45",
  "pydantic>=2.7",
]

[project.optional-dependencies]
dev = [
  "pytest>=8.0",
  "pytest-asyncio>=0.23",
]

[project.scripts]
sagano-sniper = "sagano_sniper.cli:main"

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["tests"]
```

- [ ] **Step 3: Write `.gitignore`**

```
__pycache__/
*.pyc
.venv/
.pytest_cache/
dist/
build/
*.egg-info/

# Recon artifacts may contain PII / session cookies
recon/manual-flow.har
recon/*.har

# User-specific config
config.toml
prefs.toml
```

- [ ] **Step 4: Create empty package init files**

```python
# sagano_sniper/__init__.py
```

```python
# tests/__init__.py
```

```python
# tests/conftest.py
import pytest
```

- [ ] **Step 5: Set up venv and install**

```bash
python3.11 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
playwright install chromium
```

Expected: clean install, Chromium downloads.

- [ ] **Step 6: Verify pytest discovers no tests yet**

Run: `pytest`
Expected: `no tests ran` (exit 5), or 0 collected — both fine.

- [ ] **Step 7: Commit**

```bash
git add pyproject.toml .gitignore sagano_sniper/ tests/
git commit -m "feat: scaffold sagano-sniper Python package"
```

---

## Task 2: Recon constants stub

**Files:**
- Create: `recon/__init__.py`
- Create: `recon/findings.py`
- Create: `recon/findings.md`
- Create: `sagano_sniper/recon.py`

- [ ] **Step 1: Create recon directory and README**

```bash
mkdir -p recon
```

Write `recon/findings.md`:

```markdown
# Recon findings

This file documents the manual reconnaissance pass. Run the manual booking
flow with Chrome DevTools Network tab open (Preserve log + Disable cache),
export the session as HAR to `recon/manual-flow.har`, then fill in
`recon/findings.py` with the values discovered.

## What to capture

1. Seat-map GET endpoint — URL template and response JSON shape.
2. The cookie/storage that the pay page requires (find by deletion test).
3. Redirect URL pattern for both `up` and `down` directions.
4. Form field selectors on `/booking/pay`.
5. Hold window after `/check` (wait on pay page until session expires).

Update `recon/findings.py` as values are confirmed.
```

- [ ] **Step 2: Write stub `recon/findings.py`**

```python
"""Constants discovered during manual recon (Phase 0).

STUB VALUES — replace with real findings before Phase 2 race-day run.
The script will run end-to-end against mocked Playwright with these stubs.
"""

SEAT_MAP_URL_TEMPLATE = (
    "https://common-api.sagano.linktivity.io/v1/services/{service_id}/seats"
    "?service_day={service_day}&from_station_id={from_station_id}"
    "&to_station_id={to_station_id}"
)

CHECK_URL = "https://common-api.sagano.linktivity.io/v1/reservations/check"

PAY_URL_TEMPLATE = (
    "https://ars-saganokanko.triplabo.jp/booking/pay"
    "?fromStationId={from_station_id}&toStationId={to_station_id}"
    "&serviceId={service_id}&seats={seats}&step=input_info"
    "&activityID={activity_id}&lang={lang}"
)

SEAT_PICKER_URL_TEMPLATE = (
    "https://file.sagano.linktivity.io/seat/{product_id}/{direction}"
    "?lang={lang}&date={date}&unitsCount={units}"
    "&backUrl={back_url}&redirectUrl={redirect_url}&currentStep=confirm"
)

FORM_SELECTORS: dict[str, str] = {
    "passenger_name": 'input[name="name"]',
    "passenger_email": 'input[name="email"]',
    "passenger_phone": 'input[name="phone"]',
}

HOLD_WINDOW_SECONDS = 300  # 5-minute assumption; measure during recon
```

- [ ] **Step 3: Write `sagano_sniper/recon.py`**

```python
"""Re-exports recon constants for import by the rest of the package."""
from recon.findings import (
    CHECK_URL,
    FORM_SELECTORS,
    HOLD_WINDOW_SECONDS,
    PAY_URL_TEMPLATE,
    SEAT_MAP_URL_TEMPLATE,
    SEAT_PICKER_URL_TEMPLATE,
)

__all__ = [
    "CHECK_URL",
    "FORM_SELECTORS",
    "HOLD_WINDOW_SECONDS",
    "PAY_URL_TEMPLATE",
    "SEAT_MAP_URL_TEMPLATE",
    "SEAT_PICKER_URL_TEMPLATE",
]
```

- [ ] **Step 4: Create `recon/__init__.py`**

```python
# recon/__init__.py
```

- [ ] **Step 5: Verify import works**

Run: `python -c "from sagano_sniper.recon import CHECK_URL; print(CHECK_URL)"`
Expected: `https://common-api.sagano.linktivity.io/v1/reservations/check`

- [ ] **Step 6: Commit**

```bash
git add recon/ sagano_sniper/recon.py
git commit -m "feat: add recon constants stub and re-export module"
```

---

## Task 3: Seat data models

**Files:**
- Create: `sagano_sniper/seats.py`
- Test: `tests/test_seats_models.py`

- [ ] **Step 1: Write failing test**

`tests/test_seats_models.py`:

```python
from sagano_sniper.seats import Seat, SeatMap


def test_seat_id_string_matches_redirect_format():
    seat = Seat(
        logical_car_id=4,
        seat_group_id=12,
        seat_id="A",
        arrangement_type_id=3,
        available=True,
    )
    assert seat.id_string() == "4-12-A-3"


def test_seat_map_filters_available():
    seats = [
        Seat(4, 12, "A", 3, available=True),
        Seat(4, 12, "B", 3, available=False),
        Seat(4, 12, "C", 3, available=True),
    ]
    smap = SeatMap(seats=seats)
    assert [s.seat_id for s in smap.available()] == ["A", "C"]
```

- [ ] **Step 2: Run, expect import failure**

Run: `pytest tests/test_seats_models.py -v`
Expected: FAIL — `ModuleNotFoundError` or `ImportError: Seat`.

- [ ] **Step 3: Implement minimal `sagano_sniper/seats.py`**

```python
from __future__ import annotations

from dataclasses import dataclass
from typing import Iterator


@dataclass(frozen=True)
class Seat:
    logical_car_id: int
    seat_group_id: int
    seat_id: str
    arrangement_type_id: int
    available: bool

    def id_string(self) -> str:
        return (
            f"{self.logical_car_id}-{self.seat_group_id}"
            f"-{self.seat_id}-{self.arrangement_type_id}"
        )


@dataclass(frozen=True)
class SeatMap:
    seats: list[Seat]

    def available(self) -> Iterator[Seat]:
        return (s for s in self.seats if s.available)
```

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_seats_models.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/seats.py tests/test_seats_models.py
git commit -m "feat: add Seat and SeatMap data models"
```

---

## Task 4: Pydantic config models

**Files:**
- Create: `sagano_sniper/config.py`
- Test: `tests/test_config_models.py`

- [ ] **Step 1: Write failing test**

`tests/test_config_models.py`:

```python
import pytest
from pydantic import ValidationError

from sagano_sniper.config import Passenger, Preference, RunConfig


def test_passenger_requires_name_email_phone():
    p = Passenger(name="Alice", email="a@b.com", phone="0123456789")
    assert p.name == "Alice"


def test_passenger_rejects_bad_email():
    with pytest.raises(ValidationError):
        Passenger(name="Alice", email="not-an-email", phone="0123456789")


def test_preference_seats_empty_means_any_in_group():
    pref = Preference(logical_car_id=4, seat_group_id=12, seat_ids=[])
    assert pref.seat_ids == []


def test_run_config_rejects_past_date():
    with pytest.raises(ValidationError):
        RunConfig(
            service_day="2020-01-01",
            from_station_id=1,
            to_station_id=4,
            product_id=51,
            service_id=44,
            direction="down",
            units=2,
            preferences=[],
            fallback_any=True,
            timeout_minutes=10,
            activity_id="LINKTIVITY-YRBTL",
            lang="zt",
        )


def test_run_config_accepts_future_date():
    cfg = RunConfig(
        service_day="2099-01-01",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        units=2,
        preferences=[
            Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"])
        ],
        fallback_any=True,
        timeout_minutes=10,
        activity_id="LINKTIVITY-YRBTL",
        lang="zt",
    )
    assert cfg.units == 2
    assert cfg.preferences[0].seat_ids == ["A", "B"]
```

- [ ] **Step 2: Run, expect import failure**

Run: `pytest tests/test_config_models.py -v`
Expected: FAIL — `ImportError`.

- [ ] **Step 3: Implement `sagano_sniper/config.py`**

```python
from __future__ import annotations

from datetime import date
from typing import Literal

from pydantic import BaseModel, EmailStr, Field, field_validator


class Passenger(BaseModel):
    name: str = Field(min_length=1)
    email: EmailStr
    phone: str = Field(min_length=1)


class Preference(BaseModel):
    logical_car_id: int
    seat_group_id: int
    seat_ids: list[str] = Field(
        default_factory=list,
        description="Seat letters/ids. Empty list means any seat in the group.",
    )


class RunConfig(BaseModel):
    service_day: str
    from_station_id: int
    to_station_id: int
    product_id: int
    service_id: int
    direction: Literal["up", "down"]
    units: int = Field(ge=1)
    preferences: list[Preference]
    fallback_any: bool
    timeout_minutes: int = Field(ge=1)
    activity_id: str
    lang: str

    @field_validator("service_day")
    @classmethod
    def must_be_future(cls, v: str) -> str:
        d = date.fromisoformat(v)
        if d < date.today():
            raise ValueError("service_day must be today or in the future")
        return v
```

- [ ] **Step 4: Install email-validator dependency**

EmailStr requires it. Add to `pyproject.toml` dependencies:

```toml
dependencies = [
  "playwright>=1.45",
  "pydantic[email]>=2.7",
]
```

Then: `pip install -e ".[dev]"`

- [ ] **Step 5: Run tests**

Run: `pytest tests/test_config_models.py -v`
Expected: 5 passed.

- [ ] **Step 6: Commit**

```bash
git add sagano_sniper/config.py tests/test_config_models.py pyproject.toml
git commit -m "feat: add Pydantic models for Passenger, Preference, RunConfig"
```

---

## Task 5: Config loader (TOML + CLI merge)

**Files:**
- Modify: `sagano_sniper/config.py`
- Test: `tests/test_config_loader.py`
- Create: `tests/fixtures/config.toml`
- Create: `tests/fixtures/prefs.toml`

- [ ] **Step 1: Write fixture TOML files**

`tests/fixtures/config.toml`:

```toml
[passenger]
name = "Alice Tanaka"
email = "alice@example.com"
phone = "0123456789"
```

`tests/fixtures/prefs.toml`:

```toml
fallback_any = true

[[preferences]]
logical_car_id = 4
seat_group_id = 12
seat_ids = ["A", "B"]

[[preferences]]
logical_car_id = 5
seat_group_id = 10
seat_ids = []
```

- [ ] **Step 2: Write failing test**

`tests/test_config_loader.py`:

```python
from pathlib import Path

from sagano_sniper.config import load_config, Passenger, RunConfig

FIXTURES = Path(__file__).parent / "fixtures"


def test_load_passenger():
    p = load_config(FIXTURES / "config.toml", "passenger", Passenger)
    assert p.name == "Alice Tanaka"
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
        units=2,
        prefs_path=FIXTURES / "prefs.toml",
        timeout_minutes=10,
        activity_id="LINKTIVITY-YRBTL",
        lang="zt",
    )
    assert cfg.fallback_any is True
    assert len(cfg.preferences) == 2
    assert cfg.preferences[0].seat_ids == ["A", "B"]
```

- [ ] **Step 3: Run, expect failure**

Run: `pytest tests/test_config_loader.py -v`
Expected: FAIL — `ImportError: load_config`.

- [ ] **Step 4: Extend `sagano_sniper/config.py`**

Append:

```python
import tomllib
from pathlib import Path
from typing import TypeVar

T = TypeVar("T", bound=BaseModel)


def load_config(path: Path, section: str, model: type[T]) -> T:
    data = tomllib.loads(Path(path).read_text())
    return model.model_validate(data[section])


def build_run_config(
    *,
    service_day: str,
    from_station_id: int,
    to_station_id: int,
    product_id: int,
    service_id: int,
    direction: str,
    units: int,
    prefs_path: Path,
    timeout_minutes: int,
    activity_id: str,
    lang: str,
) -> RunConfig:
    prefs_data = tomllib.loads(Path(prefs_path).read_text())
    return RunConfig(
        service_day=service_day,
        from_station_id=from_station_id,
        to_station_id=to_station_id,
        product_id=product_id,
        service_id=service_id,
        direction=direction,
        units=units,
        preferences=[Preference(**p) for p in prefs_data["preferences"]],
        fallback_any=prefs_data.get("fallback_any", False),
        timeout_minutes=timeout_minutes,
        activity_id=activity_id,
        lang=lang,
    )
```

- [ ] **Step 5: Run tests**

Run: `pytest tests/test_config_loader.py -v`
Expected: 2 passed.

- [ ] **Step 6: Commit**

```bash
git add sagano_sniper/config.py tests/test_config_loader.py tests/fixtures/
git commit -m "feat: add TOML config loader and run-config builder"
```

---

## Task 6: Picker — exact preference match

**Files:**
- Create: `sagano_sniper/picker.py`
- Test: `tests/test_picker.py`

- [ ] **Step 1: Write failing test**

`tests/test_picker.py`:

```python
from sagano_sniper.config import Preference
from sagano_sniper.picker import pick
from sagano_sniper.seats import Seat, SeatMap


def _smap(*seats: Seat) -> SeatMap:
    return SeatMap(seats=list(seats))


def test_pick_returns_seats_matching_first_preference():
    smap = _smap(
        Seat(4, 12, "A", 3, available=True),
        Seat(4, 12, "B", 3, available=True),
        Seat(4, 12, "C", 3, available=True),
    )
    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"])]
    result = pick(smap, prefs, units=2, fallback_any=False)
    assert result is not None
    assert [s.seat_id for s in result] == ["A", "B"]


def test_pick_skips_unavailable_seats_within_preference():
    smap = _smap(
        Seat(4, 12, "A", 3, available=False),
        Seat(4, 12, "B", 3, available=True),
    )
    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"])]
    result = pick(smap, prefs, units=2, fallback_any=False)
    assert result is None  # only B is available, need 2
```

- [ ] **Step 2: Run, expect import failure**

Run: `pytest tests/test_picker.py -v`
Expected: FAIL — `ImportError: pick`.

- [ ] **Step 3: Implement `sagano_sniper/picker.py`**

```python
from __future__ import annotations

from sagano_sniper.config import Preference
from sagano_sniper.seats import Seat, SeatMap


def _match_preference(
    smap: SeatMap, pref: Preference, units: int
) -> list[Seat] | None:
    in_group = [
        s
        for s in smap.available()
        if s.logical_car_id == pref.logical_car_id
        and s.seat_group_id == pref.seat_group_id
    ]
    if pref.seat_ids:
        in_group = [s for s in in_group if s.seat_id in pref.seat_ids]
    if len(in_group) >= units:
        return in_group[:units]
    return None


def pick(
    smap: SeatMap,
    prefs: list[Preference],
    units: int,
    fallback_any: bool,
) -> list[Seat] | None:
    for pref in prefs:
        match = _match_preference(smap, pref, units)
        if match:
            return match
    if fallback_any:
        any_avail = list(smap.available())
        if len(any_avail) >= units:
            return any_avail[:units]
    return None
```

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_picker.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/picker.py tests/test_picker.py
git commit -m "feat: add picker with exact-preference matching"
```

---

## Task 7: Picker — fallback through preferences and to "any"

**Files:**
- Modify: `tests/test_picker.py`

- [ ] **Step 1: Add failing tests**

Append to `tests/test_picker.py`:

```python
def test_pick_falls_through_to_second_preference():
    smap = _smap(
        Seat(4, 12, "A", 3, available=False),
        Seat(4, 12, "B", 3, available=False),
        Seat(5, 10, "A", 3, available=True),
        Seat(5, 10, "B", 3, available=True),
    )
    prefs = [
        Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"]),
        Preference(logical_car_id=5, seat_group_id=10, seat_ids=["A", "B"]),
    ]
    result = pick(smap, prefs, units=2, fallback_any=False)
    assert result is not None
    assert [(s.logical_car_id, s.seat_id) for s in result] == [(5, "A"), (5, "B")]


def test_pick_falls_back_to_any_when_no_preference_matches():
    smap = _smap(
        Seat(4, 12, "A", 3, available=False),
        Seat(9, 99, "Z", 3, available=True),
        Seat(9, 99, "Y", 3, available=True),
    )
    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A"])]
    result = pick(smap, prefs, units=2, fallback_any=True)
    assert result is not None
    assert {s.seat_id for s in result} == {"Z", "Y"}


def test_pick_returns_none_when_no_match_and_no_fallback():
    smap = _smap(Seat(4, 12, "A", 3, available=True))
    prefs = [Preference(logical_car_id=5, seat_group_id=10, seat_ids=["A"])]
    assert pick(smap, prefs, units=1, fallback_any=False) is None


def test_pick_empty_seat_ids_means_any_in_group():
    smap = _smap(
        Seat(4, 12, "X", 3, available=True),
        Seat(4, 12, "Y", 3, available=True),
    )
    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=[])]
    result = pick(smap, prefs, units=2, fallback_any=False)
    assert result is not None
    assert len(result) == 2
```

- [ ] **Step 2: Run, expect all to pass**

The current `pick` implementation already handles these cases. Verify.

Run: `pytest tests/test_picker.py -v`
Expected: 6 passed (2 prior + 4 new).

- [ ] **Step 3: Commit**

```bash
git add tests/test_picker.py
git commit -m "test: cover picker fallback paths and edge cases"
```

---

## Task 8: Seat-map fetcher (mocked Playwright)

**Files:**
- Modify: `sagano_sniper/seats.py`
- Test: `tests/test_seats_fetch.py`

- [ ] **Step 1: Write failing test**

`tests/test_seats_fetch.py`:

```python
import json
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.seats import fetch_availability


@pytest.mark.asyncio
async def test_fetch_availability_parses_seats():
    fake_response = MagicMock()
    fake_response.ok = True
    fake_response.json = AsyncMock(
        return_value={
            "seats": [
                {
                    "logical_car_id": 4,
                    "seat_group_id": 12,
                    "seat_id": "A",
                    "arrangement_type_id": 3,
                    "available": True,
                },
                {
                    "logical_car_id": 4,
                    "seat_group_id": 12,
                    "seat_id": "B",
                    "arrangement_type_id": 3,
                    "available": False,
                },
            ]
        }
    )

    fake_request = MagicMock()
    fake_request.get = AsyncMock(return_value=fake_response)

    fake_page = MagicMock()
    fake_page.request = fake_request

    smap = await fetch_availability(
        fake_page,
        service_id=44,
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
    )

    assert len(smap.seats) == 2
    assert smap.seats[0].available is True
    assert smap.seats[1].available is False
    fake_request.get.assert_awaited_once()


@pytest.mark.asyncio
async def test_fetch_availability_raises_on_http_error():
    fake_response = MagicMock()
    fake_response.ok = False
    fake_response.status = 503

    fake_page = MagicMock()
    fake_page.request = MagicMock(get=AsyncMock(return_value=fake_response))

    with pytest.raises(RuntimeError, match="503"):
        await fetch_availability(
            fake_page,
            service_id=44,
            service_day="2099-07-12",
            from_station_id=1,
            to_station_id=4,
        )
```

- [ ] **Step 2: Run, expect failure**

Run: `pytest tests/test_seats_fetch.py -v`
Expected: FAIL — `ImportError: fetch_availability`.

- [ ] **Step 3: Extend `sagano_sniper/seats.py`**

Append:

```python
from typing import Any

from sagano_sniper.recon import SEAT_MAP_URL_TEMPLATE


async def fetch_availability(
    page: Any,  # playwright.async_api.Page
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

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_seats_fetch.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/seats.py tests/test_seats_fetch.py
git commit -m "feat: add seat-map availability fetcher"
```

---

## Task 9: Flow polling loop

**Files:**
- Create: `sagano_sniper/flow.py`
- Test: `tests/test_flow_polling.py`

- [ ] **Step 1: Write failing test**

`tests/test_flow_polling.py`:

```python
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.config import Preference
from sagano_sniper.flow import poll_until_match
from sagano_sniper.seats import Seat, SeatMap


@pytest.mark.asyncio
async def test_poll_returns_seats_on_first_match():
    smap = SeatMap(
        seats=[
            Seat(4, 12, "A", 3, available=True),
            Seat(4, 12, "B", 3, available=True),
        ]
    )

    async def fake_fetch(*args, **kwargs):
        return smap

    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"])]

    result = await poll_until_match(
        page=MagicMock(),
        fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4},
        prefs=prefs,
        units=2,
        fallback_any=False,
        poll_interval=0.01,
        timeout_seconds=1.0,
    )
    assert result is not None
    assert [s.seat_id for s in result] == ["A", "B"]


@pytest.mark.asyncio
async def test_poll_returns_none_after_timeout():
    empty = SeatMap(seats=[Seat(4, 12, "A", 3, available=False)])

    async def fake_fetch(*args, **kwargs):
        return empty

    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A"])]

    result = await poll_until_match(
        page=MagicMock(),
        fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4},
        prefs=prefs,
        units=1,
        fallback_any=False,
        poll_interval=0.01,
        timeout_seconds=0.05,
    )
    assert result is None


@pytest.mark.asyncio
async def test_poll_retries_on_fetch_error():
    smap_good = SeatMap(seats=[Seat(4, 12, "A", 3, available=True)])
    calls = {"n": 0}

    async def fake_fetch(*args, **kwargs):
        calls["n"] += 1
        if calls["n"] < 3:
            raise RuntimeError("seat-map fetch failed: HTTP 503")
        return smap_good

    prefs = [Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A"])]

    result = await poll_until_match(
        page=MagicMock(),
        fetch=fake_fetch,
        fetch_kwargs={"service_id": 44, "service_day": "2099-07-12",
                      "from_station_id": 1, "to_station_id": 4},
        prefs=prefs,
        units=1,
        fallback_any=False,
        poll_interval=0.01,
        timeout_seconds=1.0,
    )
    assert result is not None
    assert calls["n"] >= 3
```

- [ ] **Step 2: Run, expect failure**

Run: `pytest tests/test_flow_polling.py -v`
Expected: FAIL — `ImportError`.

- [ ] **Step 3: Implement `sagano_sniper/flow.py`**

```python
from __future__ import annotations

import asyncio
import logging
from typing import Any, Awaitable, Callable

from sagano_sniper.config import Preference
from sagano_sniper.picker import pick
from sagano_sniper.seats import Seat, SeatMap

log = logging.getLogger(__name__)

FetchFn = Callable[..., Awaitable[SeatMap]]


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
                log.info("matched %d seats: %s",
                         len(match), [s.id_string() for s in match])
                return match
        except RuntimeError as exc:
            consecutive_errors += 1
            log.warning("fetch failed (%d in a row): %s",
                        consecutive_errors, exc)
            if consecutive_errors >= 5:
                raise RuntimeError(
                    "5 consecutive seat-map failures; aborting"
                ) from exc
        await asyncio.sleep(poll_interval)

    return None
```

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_flow_polling.py -v`
Expected: 3 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/flow.py tests/test_flow_polling.py
git commit -m "feat: add polling loop with error retry and timeout"
```

---

## Task 10: Flow — hold, navigate, fill, pause

**Files:**
- Modify: `sagano_sniper/flow.py`
- Test: `tests/test_flow_handoff.py`

- [ ] **Step 1: Write failing test**

`tests/test_flow_handoff.py`:

```python
from unittest.mock import AsyncMock, MagicMock

import pytest

from sagano_sniper.config import Passenger
from sagano_sniper.flow import hold_and_handoff
from sagano_sniper.seats import Seat


@pytest.mark.asyncio
async def test_hold_and_handoff_posts_check_then_navigates_then_fills():
    seats = [
        Seat(4, 12, "A", 3, available=True),
        Seat(4, 12, "B", 3, available=True),
    ]

    check_response = MagicMock()
    check_response.ok = True
    check_response.status = 200

    fake_request = MagicMock()
    fake_request.post = AsyncMock(return_value=check_response)
    fake_locator = MagicMock(fill=AsyncMock())

    fake_page = MagicMock()
    fake_page.request = fake_request
    fake_page.goto = AsyncMock()
    fake_page.url = "https://ars-saganokanko.triplabo.jp/booking/pay?..."
    fake_page.locator = MagicMock(return_value=fake_locator)
    fake_page.pause = AsyncMock()

    passenger = Passenger(name="Alice", email="a@b.com", phone="0123456789")

    await hold_and_handoff(
        page=fake_page,
        seats=seats,
        passenger=passenger,
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        service_day="2099-07-12",
        activity_id="LINKTIVITY-YRBTL",
        lang="zt",
    )

    # /check was POSTed with seats payload
    fake_request.post.assert_awaited_once()
    post_call = fake_request.post.await_args
    body = post_call.kwargs.get("data") or post_call.args[1]
    assert "seats" in str(body)

    # goto navigated to /booking/pay
    fake_page.goto.assert_awaited_once()
    goto_url = fake_page.goto.await_args.args[0]
    assert "booking/pay" in goto_url
    assert "4-12-A-3" in goto_url
    assert "4-12-B-3" in goto_url

    # form fields filled
    assert fake_locator.fill.await_count == 3

    # paused for user
    fake_page.pause.assert_awaited_once()


@pytest.mark.asyncio
async def test_hold_raises_on_check_failure():
    bad = MagicMock()
    bad.ok = False
    bad.status = 409
    bad.text = AsyncMock(return_value="seats taken")

    fake_page = MagicMock()
    fake_page.request = MagicMock(post=AsyncMock(return_value=bad))

    passenger = Passenger(name="Alice", email="a@b.com", phone="0123456789")

    with pytest.raises(RuntimeError, match="409"):
        await hold_and_handoff(
            page=fake_page,
            seats=[Seat(4, 12, "A", 3, available=True)],
            passenger=passenger,
            from_station_id=1,
            to_station_id=4,
            product_id=51,
            service_id=44,
            service_day="2099-07-12",
            activity_id="LINKTIVITY-YRBTL",
            lang="zt",
        )
```

- [ ] **Step 2: Run, expect failure**

Run: `pytest tests/test_flow_handoff.py -v`
Expected: FAIL — `ImportError: hold_and_handoff`.

- [ ] **Step 3: Extend `sagano_sniper/flow.py`**

Append:

```python
import json
import uuid

from sagano_sniper.config import Passenger
from sagano_sniper.recon import CHECK_URL, FORM_SELECTORS, PAY_URL_TEMPLATE


class CheckFailed(RuntimeError):
    """The /check call returned non-200 — seats were likely taken between
    poll and hold. Caller should re-enter polling."""


def _build_check_payload(
    *,
    product_id: int,
    from_station_id: int,
    service_day: str,
    service_id: int,
    seats: list[Seat],
    mutation_id: str,
) -> dict:
    return {
        "product_id": str(product_id),
        "from_station_id": str(from_station_id),
        "service": {
            "service_day": service_day,
            "service_id": str(service_id),
            "manual": {
                "seats": [
                    {
                        "logical_car_id": str(s.logical_car_id),
                        "seat_group_id": str(s.seat_group_id),
                        "seat_id": s.seat_id,
                        "arrangement_type_id": str(s.arrangement_type_id),
                    }
                    for s in seats
                ]
            },
            "mutation_id": mutation_id,
        },
    }


async def hold_and_handoff(
    *,
    page: Any,
    seats: list[Seat],
    passenger: Passenger,
    from_station_id: int,
    to_station_id: int,
    product_id: int,
    service_id: int,
    service_day: str,
    activity_id: str,
    lang: str,
) -> None:
    mutation_id = str(uuid.uuid4())
    payload = _build_check_payload(
        product_id=product_id,
        from_station_id=from_station_id,
        service_day=service_day,
        service_id=service_id,
        seats=seats,
        mutation_id=mutation_id,
    )
    resp = await page.request.post(
        CHECK_URL,
        data=json.dumps(payload),
        headers={"Content-Type": "application/json"},
    )
    if not resp.ok:
        body = ""
        try:
            body = await resp.text()
        except Exception:
            pass
        raise CheckFailed(f"/check failed: HTTP {resp.status} {body}")

    seats_param = ",".join(s.id_string() for s in seats)
    pay_url = PAY_URL_TEMPLATE.format(
        from_station_id=from_station_id,
        to_station_id=to_station_id,
        service_id=service_id,
        seats=seats_param,
        activity_id=activity_id,
        lang=lang,
    )
    await page.goto(pay_url)

    if "booking/pay" not in page.url:
        raise RuntimeError(
            f"redirect landed off-target: {page.url} — session may be invalid"
        )

    for field, value in (
        ("passenger_name", passenger.name),
        ("passenger_email", passenger.email),
        ("passenger_phone", passenger.phone),
    ):
        selector = FORM_SELECTORS.get(field)
        if not selector:
            raise RuntimeError(f"selector for {field} missing in recon findings")
        await page.locator(selector).fill(value)

    log.info("form filled; pausing for user to verify and submit")
    await page.pause()
```

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_flow_handoff.py -v`
Expected: 2 passed.

- [ ] **Step 5: Commit**

```bash
git add sagano_sniper/flow.py tests/test_flow_handoff.py
git commit -m "feat: add hold + redirect + form fill + pause handoff"
```

---

## Task 11: Flow entrypoint (launches Playwright)

**Files:**
- Modify: `sagano_sniper/flow.py`
- Test: `tests/test_flow_run.py`

- [ ] **Step 1: Write failing test**

`tests/test_flow_run.py`:

```python
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from sagano_sniper.config import Passenger, Preference, RunConfig
from sagano_sniper.flow import run
from sagano_sniper.seats import Seat, SeatMap


def _config() -> RunConfig:
    return RunConfig(
        service_day="2099-07-12",
        from_station_id=1,
        to_station_id=4,
        product_id=51,
        service_id=44,
        direction="down",
        units=2,
        preferences=[
            Preference(logical_car_id=4, seat_group_id=12, seat_ids=["A", "B"])
        ],
        fallback_any=False,
        timeout_minutes=1,
        activity_id="LINKTIVITY-YRBTL",
        lang="zt",
    )


@pytest.mark.asyncio
async def test_run_orchestrates_launch_poll_hold():
    smap = SeatMap(
        seats=[
            Seat(4, 12, "A", 3, available=True),
            Seat(4, 12, "B", 3, available=True),
        ]
    )

    fake_page = MagicMock()
    fake_page.goto = AsyncMock()
    fake_page.url = "https://ars-saganokanko.triplabo.jp/booking/pay?ok"
    fake_page.locator = MagicMock(return_value=MagicMock(fill=AsyncMock()))
    fake_page.pause = AsyncMock()

    check_resp = MagicMock(ok=True, status=200)
    fake_page.request = MagicMock(
        get=AsyncMock(
            return_value=MagicMock(
                ok=True, json=AsyncMock(return_value={"seats": [
                    {"logical_car_id": 4, "seat_group_id": 12, "seat_id": "A",
                     "arrangement_type_id": 3, "available": True},
                    {"logical_car_id": 4, "seat_group_id": 12, "seat_id": "B",
                     "arrangement_type_id": 3, "available": True},
                ]}),
            )
        ),
        post=AsyncMock(return_value=check_resp),
    )

    fake_context = MagicMock(new_page=AsyncMock(return_value=fake_page))
    fake_browser = MagicMock(
        new_context=AsyncMock(return_value=fake_context),
        close=AsyncMock(),
    )
    fake_pw = MagicMock()
    fake_pw.chromium.launch = AsyncMock(return_value=fake_browser)

    @pytest.fixture
    def _noop():
        pass

    class _PWCtx:
        async def __aenter__(self):
            return fake_pw

        async def __aexit__(self, *a):
            return False

    passenger = Passenger(name="Alice", email="a@b.com", phone="0123456789")

    with patch("sagano_sniper.flow.async_playwright", return_value=_PWCtx()):
        await run(
            config=_config(),
            passenger=passenger,
            poll_interval=0.01,
        )

    fake_pw.chromium.launch.assert_awaited_once()
    fake_page.goto.assert_awaited()  # picker URL + pay URL
    fake_page.pause.assert_awaited_once()
```

- [ ] **Step 2: Run, expect failure**

Run: `pytest tests/test_flow_run.py -v`
Expected: FAIL — `ImportError: run`.

- [ ] **Step 3: Extend `sagano_sniper/flow.py`**

Append:

```python
from playwright.async_api import async_playwright

from sagano_sniper.config import RunConfig
from sagano_sniper.recon import SEAT_PICKER_URL_TEMPLATE
from sagano_sniper.seats import fetch_availability


def _picker_url(config: RunConfig) -> str:
    return SEAT_PICKER_URL_TEMPLATE.format(
        product_id=config.product_id,
        direction=config.direction,
        lang=config.lang,
        date=config.service_day,
        units=config.units,
        back_url="",
        redirect_url="",
    )


async def run(
    *,
    config: RunConfig,
    passenger: Passenger,
    poll_interval: float = 1.0,
) -> None:
    picker_url = _picker_url(config)

    async with async_playwright() as pw:
        browser = await pw.chromium.launch(headless=False)
        try:
            context = await browser.new_context()
            page = await context.new_page()
            await page.goto(picker_url)

            for attempt in range(10):
                seats = await poll_until_match(
                    page=page,
                    fetch=fetch_availability,
                    fetch_kwargs={
                        "service_id": config.service_id,
                        "service_day": config.service_day,
                        "from_station_id": config.from_station_id,
                        "to_station_id": config.to_station_id,
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
                    await hold_and_handoff(
                        page=page,
                        seats=seats,
                        passenger=passenger,
                        from_station_id=config.from_station_id,
                        to_station_id=config.to_station_id,
                        product_id=config.product_id,
                        service_id=config.service_id,
                        service_day=config.service_day,
                        activity_id=config.activity_id,
                        lang=config.lang,
                    )
                    return  # handoff complete, page is paused
                except CheckFailed as exc:
                    log.warning(
                        "/check failed on attempt %d/10; re-entering polling: %s",
                        attempt + 1, exc,
                    )
                    continue
            raise RuntimeError("/check failed 10 times in a row; aborting")
        finally:
            await browser.close()
```

- [ ] **Step 4: Run tests**

Run: `pytest tests/test_flow_run.py -v`
Expected: 1 passed.

- [ ] **Step 5: Run full test suite**

Run: `pytest -v`
Expected: all green.

- [ ] **Step 6: Commit**

```bash
git add sagano_sniper/flow.py tests/test_flow_run.py
git commit -m "feat: add top-level flow.run orchestrator"
```

---

## Task 12: CLI + examples + README

**Files:**
- Create: `sagano_sniper/cli.py`
- Create: `config.toml.example`
- Create: `prefs.toml.example`
- Create: `README.md`

- [ ] **Step 1: Implement CLI**

`sagano_sniper/cli.py`:

```python
from __future__ import annotations

import argparse
import asyncio
import logging
from pathlib import Path

from sagano_sniper.config import (
    Passenger,
    build_run_config,
    load_config,
)
from sagano_sniper.flow import run


def _parse_args() -> argparse.Namespace:
    p = argparse.ArgumentParser(prog="sagano-sniper")
    sub = p.add_subparsers(dest="cmd", required=True)
    r = sub.add_parser("run", help="run the sniper")
    r.add_argument("--config", type=Path, required=True,
                   help="TOML with [passenger] section")
    r.add_argument("--prefs", type=Path, required=True,
                   help="TOML with [[preferences]] entries and fallback_any")
    r.add_argument("--date", required=True, help="service day YYYY-MM-DD")
    r.add_argument("--from-station", type=int, required=True)
    r.add_argument("--to-station", type=int, required=True)
    r.add_argument("--product", type=int, required=True)
    r.add_argument("--service-id", type=int, required=True)
    r.add_argument("--direction", choices=["up", "down"], required=True)
    r.add_argument("--units", type=int, required=True)
    r.add_argument("--activity-id", required=True)
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
    passenger = load_config(args.config, "passenger", Passenger)
    config = build_run_config(
        service_day=args.date,
        from_station_id=args.from_station,
        to_station_id=args.to_station,
        product_id=args.product,
        service_id=args.service_id,
        direction=args.direction,
        units=args.units,
        prefs_path=args.prefs,
        timeout_minutes=args.timeout,
        activity_id=args.activity_id,
        lang=args.lang,
    )
    asyncio.run(
        run(config=config, passenger=passenger, poll_interval=args.poll_interval)
    )
```

- [ ] **Step 2: Write `config.toml.example`**

```toml
[passenger]
name = "Your Name"
email = "you@example.com"
phone = "0123456789"
```

- [ ] **Step 3: Write `prefs.toml.example`**

```toml
# If true and no preference matches, grab any N available seats.
fallback_any = true

# Try preferences in order. seat_ids = [] means any seat in the group.
[[preferences]]
logical_car_id = 4
seat_group_id = 12
seat_ids = ["A", "B"]

[[preferences]]
logical_car_id = 5
seat_group_id = 10
seat_ids = []
```

- [ ] **Step 4: Write `README.md`**

````markdown
# sagano-sniper

CLI that snipes Sagano Romantic Train seats the instant they're released,
fills the passenger form, and pauses for you to verify and pay.

## Setup

```bash
python3.11 -m venv .venv
source .venv/bin/activate
pip install -e ".[dev]"
playwright install chromium
```

## Configuration

Copy the example files and fill in your details:

```bash
cp config.toml.example config.toml   # passenger info
cp prefs.toml.example prefs.toml     # seat preferences
```

## Recon (one-time, see `recon/findings.md`)

Before a real run, walk a manual booking with DevTools open and update
`recon/findings.py` with the discovered URLs, cookies, and form selectors.

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
  --units 2 \
  --activity-id LINKTIVITY-YRBTL \
  --timeout 10 \
  --poll-interval 1.0
```

A Chromium window opens. The script polls until your preferred seats appear,
holds them via `/check`, navigates to the pay page, fills your details, and
pauses. Verify the form, then click Pay yourself.

## Tests

```bash
pytest -v
```
````

- [ ] **Step 5: Verify CLI is registered**

Run: `sagano-sniper run --help`
Expected: help text prints with all `--` flags.

- [ ] **Step 6: Run full test suite**

Run: `pytest -v`
Expected: all green.

- [ ] **Step 7: Commit**

```bash
git add sagano_sniper/cli.py config.toml.example prefs.toml.example README.md
git commit -m "feat: add CLI entrypoint, example configs, and README"
```

---

## Wrap-up — Phase 2 preparation

After all tasks above are complete:

1. User runs the recon flow (per `recon/findings.md`), saves HAR, and updates `recon/findings.py` with real URLs, cookie names, form selectors, and measured hold window.
2. User runs an end-to-end dry test against a low-demand date with plenty of seats. The script should reach the pay page with the form filled.
3. On release day, user launches 5 minutes before the official release time. Script polls, fires when seats appear, hands off browser.

No code changes needed for Phase 2 unless recon revealed a mismatch with stub assumptions (e.g. seat-map response shape differs). In that case, update `sagano_sniper/seats.py:fetch_availability` parsing and re-run tests.
