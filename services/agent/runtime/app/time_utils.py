from __future__ import annotations

from datetime import datetime, timezone


def utc_now() -> datetime:
    """Return naive UTC for MySQL DATETIME columns, which do not carry timezone data."""
    return datetime.now(timezone.utc).replace(tzinfo=None)


def utc_from_timestamp(timestamp: float) -> datetime:
    """Convert a Unix timestamp to naive UTC for internal persistence."""
    return datetime.fromtimestamp(timestamp, timezone.utc).replace(tzinfo=None)
