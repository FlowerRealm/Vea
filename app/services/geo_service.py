from __future__ import annotations

import hashlib
from datetime import datetime
from typing import Sequence

import httpx
from sqlmodel import Session, select

from ..models import GeoResource, compute_next_update
from ..schemas import GeoResourceCreate, GeoResourceUpdate


class GeoSyncError(Exception):
    pass


def list_resources(session: Session) -> Sequence[GeoResource]:
    return session.exec(select(GeoResource).order_by(GeoResource.id)).all()


def get_resource(session: Session, resource_id: int) -> GeoResource | None:
    return session.get(GeoResource, resource_id)


def create_resource(session: Session, data: GeoResourceCreate) -> GeoResource:
    payload = data.dict()
    resource = GeoResource(**payload)
    if resource.auto_update:
        resource.next_update_at = compute_next_update(resource.update_interval_minutes)
    session.add(resource)
    session.commit()
    session.refresh(resource)
    return resource


def update_resource(session: Session, resource: GeoResource, data: GeoResourceUpdate) -> GeoResource:
    payload = data.dict(exclude_unset=True)
    for key, value in payload.items():
        setattr(resource, key, value)
    if resource.auto_update:
        resource.next_update_at = compute_next_update(resource.update_interval_minutes)
    else:
        resource.next_update_at = None
    session.add(resource)
    session.commit()
    session.refresh(resource)
    return resource


def delete_resource(session: Session, resource: GeoResource) -> None:
    session.delete(resource)
    session.commit()


def _download(url: str) -> bytes:
    try:
        with httpx.Client(timeout=30) as client:
            response = client.get(url)
            response.raise_for_status()
            return response.content
    except httpx.HTTPError as exc:
        raise GeoSyncError(str(exc)) from exc


def refresh_resource(session: Session, resource: GeoResource) -> GeoResource:
    content = _download(resource.source_url)
    resource.checksum = hashlib.sha256(content).hexdigest()
    resource.last_synced_at = datetime.utcnow()
    if resource.auto_update:
        resource.next_update_at = compute_next_update(resource.update_interval_minutes)
    session.add(resource)
    session.commit()
    session.refresh(resource)
    return resource


def refresh_due_resources(session: Session, now: datetime | None = None) -> list[int]:
    now = now or datetime.utcnow()
    stmt = select(GeoResource).where(
        GeoResource.auto_update.is_(True),
        GeoResource.next_update_at.is_not(None),
        GeoResource.next_update_at <= now,
    )
    updated: list[int] = []
    for resource in session.exec(stmt):
        try:
            refresh_resource(session, resource)
            updated.append(resource.id)
        except GeoSyncError:
            resource.next_update_at = compute_next_update(resource.update_interval_minutes)
            session.add(resource)
            session.commit()
    return updated
