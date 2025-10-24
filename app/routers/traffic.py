from __future__ import annotations

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session

from ..database import get_session
from ..schemas import (
    SystemSettingsRead,
    SystemSettingsUpdate,
    TrafficRuleCreate,
    TrafficRuleRead,
    TrafficRuleUpdate,
)
from ..services import traffic_service

router = APIRouter(prefix="/traffic", tags=["traffic"])


@router.get("/rules", response_model=list[TrafficRuleRead])
def list_rules(session: Session = Depends(get_session)) -> list[TrafficRuleRead]:
    return list(traffic_service.list_rules(session))


@router.post("/rules", response_model=TrafficRuleRead, status_code=status.HTTP_201_CREATED)
def create_rule(
    payload: TrafficRuleCreate,
    session: Session = Depends(get_session),
) -> TrafficRuleRead:
    return traffic_service.create_rule(session, payload)


@router.put("/rules/{rule_id}", response_model=TrafficRuleRead)
def update_rule(
    rule_id: int,
    payload: TrafficRuleUpdate,
    session: Session = Depends(get_session),
) -> TrafficRuleRead:
    rule = traffic_service.get_rule(session, rule_id)
    if not rule:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Rule not found")
    return traffic_service.update_rule(session, rule, payload)


@router.delete("/rules/{rule_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_rule(rule_id: int, session: Session = Depends(get_session)) -> None:
    rule = traffic_service.get_rule(session, rule_id)
    if not rule:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Rule not found")
    traffic_service.delete_rule(session, rule)


@router.get("/settings", response_model=SystemSettingsRead)
def get_settings(session: Session = Depends(get_session)) -> SystemSettingsRead:
    return traffic_service.get_settings(session)


@router.put("/settings", response_model=SystemSettingsRead)
def update_settings(
    payload: SystemSettingsUpdate,
    session: Session = Depends(get_session),
) -> SystemSettingsRead:
    return traffic_service.update_settings(session, payload)
