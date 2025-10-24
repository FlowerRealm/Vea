from __future__ import annotations

from datetime import datetime
from typing import Iterable, Sequence

import httpx
from sqlmodel import Session, select

from ..models import ConfigProfile, compute_next_update
from ..schemas import ConfigProfileCreate, ConfigProfileUpdate
from ..utils.importer import normalize_config


class ConfigSyncError(Exception):
    pass


def list_configs(session: Session) -> Sequence[ConfigProfile]:
    return session.exec(select(ConfigProfile).order_by(ConfigProfile.id)).all()


def get_config(session: Session, config_id: int) -> ConfigProfile | None:
    return session.get(ConfigProfile, config_id)


def create_config(session: Session, data: ConfigProfileCreate) -> ConfigProfile:
    payload = data.dict()
    normalized = normalize_config(data.format, data.raw_content)
    profile = ConfigProfile(**payload, normalized=normalized)
    if profile.auto_update:
        profile.next_update_at = compute_next_update(profile.update_interval_minutes)
    session.add(profile)
    session.commit()
    session.refresh(profile)
    return profile


def update_config(session: Session, profile: ConfigProfile, data: ConfigProfileUpdate) -> ConfigProfile:
    payload = data.dict(exclude_unset=True)
    if "raw_content" in payload:
        profile.raw_content = payload.pop("raw_content")
        profile.normalized = normalize_config(profile.format, profile.raw_content)
        profile.last_synced_at = datetime.utcnow()
    for key, value in payload.items():
        setattr(profile, key, value)
    if profile.auto_update:
        profile.next_update_at = compute_next_update(profile.update_interval_minutes)
    else:
        profile.next_update_at = None
    session.add(profile)
    session.commit()
    session.refresh(profile)
    return profile


def delete_config(session: Session, profile: ConfigProfile) -> None:
    session.delete(profile)
    session.commit()


def _download(url: str) -> str:
    try:
        with httpx.Client(timeout=20) as client:
            response = client.get(url)
            response.raise_for_status()
            return response.text
    except httpx.HTTPError as exc:
        raise ConfigSyncError(str(exc)) from exc


def refresh_profile_from_source(session: Session, profile: ConfigProfile) -> ConfigProfile:
    if not profile.source_url:
        raise ConfigSyncError("profile does not have a source_url")
    raw_content = _download(profile.source_url)
    profile.raw_content = raw_content
    profile.normalized = normalize_config(profile.format, raw_content)
    profile.last_synced_at = datetime.utcnow()
    if profile.auto_update:
        profile.next_update_at = compute_next_update(profile.update_interval_minutes)
    session.add(profile)
    session.commit()
    session.refresh(profile)
    return profile


def refresh_due_profiles(session: Session, now: datetime | None = None) -> list[int]:
    now = now or datetime.utcnow()
    stmt = select(ConfigProfile).where(
        ConfigProfile.auto_update.is_(True),
        ConfigProfile.next_update_at.is_not(None),
        ConfigProfile.next_update_at <= now,
    )
    updated: list[int] = []
    for profile in session.exec(stmt):
        try:
            refresh_profile_from_source(session, profile)
            updated.append(profile.id)
        except ConfigSyncError:
            profile.next_update_at = compute_next_update(profile.update_interval_minutes)
            session.add(profile)
            session.commit()
    return updated


def bulk_update_usage(session: Session, entries: Iterable[tuple[int, int, int]]) -> None:
    now = datetime.utcnow()
    for config_id, upstream, downstream in entries:
        profile = session.get(ConfigProfile, config_id)
        if not profile:
            continue
        profile.upstream_traffic = upstream
        profile.downstream_traffic = downstream
        profile.last_synced_at = now
        session.add(profile)
    session.commit()
