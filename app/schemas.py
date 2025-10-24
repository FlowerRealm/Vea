from __future__ import annotations

from datetime import datetime
from typing import Any, Dict, Optional

from pydantic import BaseModel, Field, HttpUrl, validator

from .models import ConfigFormat, ProtocolType, TrafficMatchType


class NodeBase(BaseModel):
    name: str
    address: str
    port: int = Field(gt=0, lt=65536)
    protocol: ProtocolType
    settings: Dict[str, Any] = Field(default_factory=dict)
    tags: list[str] = Field(default_factory=list)
    total_quota: Optional[int] = Field(default=None, ge=0)


class NodeCreate(NodeBase):
    pass


class NodeUpdate(BaseModel):
    name: Optional[str] = None
    address: Optional[str] = None
    port: Optional[int] = Field(default=None, gt=0, lt=65536)
    protocol: Optional[ProtocolType] = None
    settings: Optional[Dict[str, Any]] = None
    tags: Optional[list[str]] = None
    is_active: Optional[bool] = None
    total_quota: Optional[int] = Field(default=None, ge=0)
    upstream_traffic: Optional[int] = Field(default=None, ge=0)
    downstream_traffic: Optional[int] = Field(default=None, ge=0)


class NodeRead(NodeBase):
    id: int
    created_at: datetime
    updated_at: datetime
    upstream_traffic: int
    downstream_traffic: int
    last_latency_ms: Optional[float]
    last_speed_mb_s: Optional[float]
    is_active: bool

    class Config:
        orm_mode = True


class TrafficUsageUpdate(BaseModel):
    upstream: int = Field(ge=0)
    downstream: int = Field(ge=0)


class ConfigProfileBase(BaseModel):
    name: str
    format: ConfigFormat
    raw_content: str
    source_url: Optional[HttpUrl] = None
    auto_update: bool = True
    update_interval_minutes: int = Field(default=60, ge=5, le=1440)
    expire_at: Optional[datetime] = None
    total_quota: Optional[int] = Field(default=None, ge=0)

    @validator("raw_content")
    def ensure_content(cls, value: str) -> str:
        if not value:
            raise ValueError("raw_content cannot be empty")
        return value


class ConfigProfileCreate(ConfigProfileBase):
    pass


class ConfigProfileUpdate(BaseModel):
    name: Optional[str] = None
    raw_content: Optional[str] = None
    source_url: Optional[HttpUrl] = None
    auto_update: Optional[bool] = None
    update_interval_minutes: Optional[int] = Field(default=None, ge=5, le=1440)
    expire_at: Optional[datetime] = None
    total_quota: Optional[int] = Field(default=None, ge=0)
    upstream_traffic: Optional[int] = Field(default=None, ge=0)
    downstream_traffic: Optional[int] = Field(default=None, ge=0)


class ConfigProfileRead(ConfigProfileBase):
    id: int
    normalized: Dict[str, Any]
    last_synced_at: Optional[datetime]
    next_update_at: Optional[datetime]
    upstream_traffic: int
    downstream_traffic: int

    class Config:
        orm_mode = True


class GeoResourceBase(BaseModel):
    name: str
    resource_type: str
    source_url: HttpUrl
    auto_update: bool = True
    update_interval_minutes: int = Field(default=1440, ge=30, le=10080)


class GeoResourceCreate(GeoResourceBase):
    pass


class GeoResourceUpdate(BaseModel):
    name: Optional[str] = None
    source_url: Optional[HttpUrl] = None
    auto_update: Optional[bool] = None
    update_interval_minutes: Optional[int] = Field(default=None, ge=30, le=10080)


class GeoResourceRead(GeoResourceBase):
    id: int
    next_update_at: Optional[datetime]
    last_synced_at: Optional[datetime]
    checksum: Optional[str]

    class Config:
        orm_mode = True


class TrafficRuleBase(BaseModel):
    name: str
    match_type: TrafficMatchType
    match_value: str
    priority: int = Field(default=100, ge=0, le=1000)
    node_id: int
    description: Optional[str] = None


class TrafficRuleCreate(TrafficRuleBase):
    pass


class TrafficRuleUpdate(BaseModel):
    name: Optional[str] = None
    match_type: Optional[TrafficMatchType] = None
    match_value: Optional[str] = None
    priority: Optional[int] = Field(default=None, ge=0, le=1000)
    node_id: Optional[int] = None
    description: Optional[str] = None


class TrafficRuleRead(TrafficRuleBase):
    id: int

    class Config:
        orm_mode = True


class SystemSettingsRead(BaseModel):
    dns: Dict[str, Any]
    routing: Dict[str, Any]
    telemetry: Dict[str, Any]

    class Config:
        orm_mode = True


class SystemSettingsUpdate(BaseModel):
    dns: Optional[Dict[str, Any]] = None
    routing: Optional[Dict[str, Any]] = None
    telemetry: Optional[Dict[str, Any]] = None


class SpeedTestRequest(BaseModel):
    test_url: HttpUrl = Field(default="https://speed.cloudflare.com/__down?bytes=1000000")
    download_bytes: int = Field(default=1_000_000, ge=100_000, le=50_000_000)


class SpeedTestResult(BaseModel):
    latency_ms: Optional[float]
    download_speed_mb_s: Optional[float]
    tested_at: datetime


class LatencyTestResult(BaseModel):
    latency_ms: Optional[float]
    tested_at: datetime
