from __future__ import annotations

from datetime import datetime, timedelta
from enum import Enum
from typing import Any, Dict, Optional

from sqlalchemy import JSON, Column
from sqlmodel import Field, SQLModel


class ProtocolType(str, Enum):
    VLESS = "vless"
    TROJAN = "trojan"
    SHADOWSOCKS = "shadowsocks"
    VMESS = "vmess"
    HYSTERIA2 = "hysteria2"


class ConfigFormat(str, Enum):
    XRAY_JSON = "xray-json"
    SING_BOX_JSON = "sing-box-json"
    V2RAYN = "v2rayn"
    CLASH = "clash"
    RAW = "raw"


class TrafficMatchType(str, Enum):
    DOMAIN = "domain"
    IP = "ip"
    PORT = "port"
    PROCESS = "process"
    TAG = "tag"


class Node(SQLModel, table=True):
    __tablename__ = "nodes"

    id: Optional[int] = Field(default=None, primary_key=True)
    name: str = Field(index=True)
    address: str
    port: int
    protocol: ProtocolType
    created_at: datetime = Field(default_factory=datetime.utcnow)
    updated_at: datetime = Field(default_factory=datetime.utcnow)
    settings: Dict[str, Any] = Field(
        default_factory=dict,
        sa_column=Column(JSON, nullable=False, server_default="{}"),
    )
    tags: list[str] = Field(
        default_factory=list,
        sa_column=Column(JSON, nullable=False, server_default="[]"),
    )
    upstream_traffic: int = Field(default=0, description="Bytes uploaded")
    downstream_traffic: int = Field(default=0, description="Bytes downloaded")
    total_quota: Optional[int] = Field(default=None, description="Quota in bytes")
    last_latency_ms: Optional[float] = None
    last_speed_mb_s: Optional[float] = None
    is_active: bool = Field(default=True, index=True)


class ConfigProfile(SQLModel, table=True):
    __tablename__ = "config_profiles"

    id: Optional[int] = Field(default=None, primary_key=True)
    name: str
    format: ConfigFormat = Field(index=True)
    raw_content: str
    normalized: Dict[str, Any] = Field(
        default_factory=dict,
        sa_column=Column(JSON, nullable=False, server_default="{}"),
    )
    source_url: Optional[str] = Field(default=None, index=True)
    auto_update: bool = Field(default=True)
    update_interval_minutes: int = Field(default=60)
    next_update_at: Optional[datetime] = Field(default=None, index=True)
    last_synced_at: Optional[datetime] = None
    expire_at: Optional[datetime] = None
    upstream_traffic: int = Field(default=0)
    downstream_traffic: int = Field(default=0)
    total_quota: Optional[int] = Field(default=None)


class GeoResource(SQLModel, table=True):
    __tablename__ = "geo_resources"

    id: Optional[int] = Field(default=None, primary_key=True)
    name: str = Field(index=True)
    resource_type: str = Field(index=True)
    source_url: str
    auto_update: bool = Field(default=True)
    update_interval_minutes: int = Field(default=1440)
    next_update_at: Optional[datetime] = None
    last_synced_at: Optional[datetime] = None
    checksum: Optional[str] = None


class TrafficRule(SQLModel, table=True):
    __tablename__ = "traffic_rules"

    id: Optional[int] = Field(default=None, primary_key=True)
    name: str
    match_type: TrafficMatchType
    match_value: str
    priority: int = Field(default=100, index=True)
    node_id: int = Field(foreign_key="nodes.id")
    description: Optional[str] = None


class SystemSettings(SQLModel, table=True):
    __tablename__ = "system_settings"

    id: Optional[int] = Field(default=1, primary_key=True)
    dns: Dict[str, Any] = Field(
        default_factory=lambda: {
            "servers": ["https://1.1.1.1/dns-query"],
            "strategy": "prefer_ipv4",
        },
        sa_column=Column(JSON, nullable=False),
    )
    routing: Dict[str, Any] = Field(
        default_factory=lambda: {
            "default_node_id": None,
            "rules": [],
        },
        sa_column=Column(JSON, nullable=False),
    )
    telemetry: Dict[str, Any] = Field(
        default_factory=lambda: {
            "auto_latency_test": True,
            "latency_interval_minutes": 10,
        },
        sa_column=Column(JSON, nullable=False),
    )


def compute_next_update(minutes: int) -> datetime:
    return datetime.utcnow() + timedelta(minutes=minutes)
