from __future__ import annotations

from typing import Sequence

from sqlmodel import Session, select

from ..models import SystemSettings, TrafficRule
from ..schemas import SystemSettingsUpdate, TrafficRuleCreate, TrafficRuleUpdate


def list_rules(session: Session) -> Sequence[TrafficRule]:
    return session.exec(select(TrafficRule).order_by(TrafficRule.priority, TrafficRule.id)).all()


def get_rule(session: Session, rule_id: int) -> TrafficRule | None:
    return session.get(TrafficRule, rule_id)


def create_rule(session: Session, data: TrafficRuleCreate) -> TrafficRule:
    rule = TrafficRule(**data.dict())
    session.add(rule)
    session.commit()
    session.refresh(rule)
    return rule


def update_rule(session: Session, rule: TrafficRule, data: TrafficRuleUpdate) -> TrafficRule:
    payload = data.dict(exclude_unset=True)
    for key, value in payload.items():
        setattr(rule, key, value)
    session.add(rule)
    session.commit()
    session.refresh(rule)
    return rule


def delete_rule(session: Session, rule: TrafficRule) -> None:
    session.delete(rule)
    session.commit()


def get_settings(session: Session) -> SystemSettings:
    settings = session.get(SystemSettings, 1)
    if not settings:
        settings = SystemSettings()
        session.add(settings)
        session.commit()
        session.refresh(settings)
    return settings


def update_settings(session: Session, data: SystemSettingsUpdate) -> SystemSettings:
    settings = get_settings(session)
    payload = data.dict(exclude_unset=True)
    for key, value in payload.items():
        current = getattr(settings, key)
        if isinstance(current, dict) and isinstance(value, dict):
            current.update(value)
        else:
            setattr(settings, key, value)
    session.add(settings)
    session.commit()
    session.refresh(settings)
    return settings
