from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session

from ..database import get_session
from ..schemas import (
    ConfigProfileCreate,
    ConfigProfileRead,
    ConfigProfileUpdate,
    TrafficUsageUpdate,
)
from ..services import config_service

router = APIRouter(prefix="/configs", tags=["configs"])


@router.get("/", response_model=list[ConfigProfileRead])
def list_configs(session: Session = Depends(get_session)) -> list[ConfigProfileRead]:
    return list(config_service.list_configs(session))


@router.post("/", response_model=ConfigProfileRead, status_code=status.HTTP_201_CREATED)
def create_config(
    payload: ConfigProfileCreate,
    session: Session = Depends(get_session),
) -> ConfigProfileRead:
    return config_service.create_config(session, payload)


@router.get("/{config_id}", response_model=ConfigProfileRead)
def get_config(config_id: int, session: Session = Depends(get_session)) -> ConfigProfileRead:
    profile = config_service.get_config(session, config_id)
    if not profile:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Config not found")
    return profile


@router.put("/{config_id}", response_model=ConfigProfileRead)
def update_config(
    config_id: int,
    payload: ConfigProfileUpdate,
    session: Session = Depends(get_session),
) -> ConfigProfileRead:
    profile = config_service.get_config(session, config_id)
    if not profile:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Config not found")
    return config_service.update_config(session, profile, payload)


@router.delete("/{config_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_config(config_id: int, session: Session = Depends(get_session)) -> None:
    profile = config_service.get_config(session, config_id)
    if not profile:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Config not found")
    config_service.delete_config(session, profile)


@router.post("/{config_id}/refresh", response_model=ConfigProfileRead)
def refresh_config(config_id: int, session: Session = Depends(get_session)) -> ConfigProfileRead:
    profile = config_service.get_config(session, config_id)
    if not profile:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Config not found")
    return config_service.refresh_profile_from_source(session, profile)


@router.post("/{config_id}/usage", response_model=ConfigProfileRead)
def update_usage(
    config_id: int,
    payload: TrafficUsageUpdate,
    session: Session = Depends(get_session),
) -> ConfigProfileRead:
    profile = config_service.get_config(session, config_id)
    if not profile:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Config not found")
    profile.upstream_traffic = payload.upstream
    profile.downstream_traffic = payload.downstream
    profile.last_synced_at = datetime.utcnow()
    session.add(profile)
    session.commit()
    session.refresh(profile)
    return profile
