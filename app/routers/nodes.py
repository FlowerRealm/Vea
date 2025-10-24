from __future__ import annotations

from datetime import datetime

from fastapi import APIRouter, Depends, HTTPException, status
from sqlmodel import Session

from ..database import get_session
from ..models import Node
from ..schemas import (
    LatencyTestResult,
    NodeCreate,
    NodeRead,
    NodeUpdate,
    SpeedTestRequest,
    SpeedTestResult,
    TrafficUsageUpdate,
)
from ..services import node_service
from ..services.metrics import measure_latency, measure_speed

router = APIRouter(prefix="/nodes", tags=["nodes"])


@router.get("/", response_model=list[NodeRead])
def list_nodes(session: Session = Depends(get_session)) -> list[Node]:
    return list(node_service.list_nodes(session))


@router.post("/", response_model=NodeRead, status_code=status.HTTP_201_CREATED)
def create_node(data: NodeCreate, session: Session = Depends(get_session)) -> Node:
    return node_service.create_node(session, data)


@router.get("/{node_id}", response_model=NodeRead)
def get_node(node_id: int, session: Session = Depends(get_session)) -> Node:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    return node


@router.put("/{node_id}", response_model=NodeRead)
def update_node(node_id: int, data: NodeUpdate, session: Session = Depends(get_session)) -> Node:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    return node_service.update_node(session, node, data)


@router.delete("/{node_id}", status_code=status.HTTP_204_NO_CONTENT)
def delete_node(node_id: int, session: Session = Depends(get_session)) -> None:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    node_service.delete_node(session, node)


@router.post("/{node_id}/usage", response_model=NodeRead)
def update_usage(
    node_id: int,
    payload: TrafficUsageUpdate,
    session: Session = Depends(get_session),
) -> Node:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    return node_service.record_usage(session, node, payload.upstream, payload.downstream)


@router.post("/{node_id}/latency-test", response_model=LatencyTestResult)
async def latency_test(node_id: int, session: Session = Depends(get_session)) -> LatencyTestResult:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    result = await measure_latency(node.address, node.port)
    node.last_latency_ms = result.latency_ms
    node.updated_at = datetime.utcnow()
    session.add(node)
    session.commit()
    tested_at = datetime.utcnow()
    return LatencyTestResult(latency_ms=result.latency_ms, tested_at=tested_at)


@router.post("/{node_id}/speed-test", response_model=SpeedTestResult)
async def speed_test(
    node_id: int,
    payload: SpeedTestRequest,
    session: Session = Depends(get_session),
) -> SpeedTestResult:
    node = node_service.get_node(session, node_id)
    if not node:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="Node not found")
    proxy = node.settings.get("http_proxy") if isinstance(node.settings, dict) else None
    result = await measure_speed(
        node.address,
        node.port,
        payload.test_url,
        payload.download_bytes,
        proxy=proxy,
    )
    node.last_latency_ms = result.latency_ms
    node.last_speed_mb_s = result.download_speed_mb_s
    node.updated_at = datetime.utcnow()
    session.add(node)
    session.commit()
    tested_at = datetime.utcnow()
    return SpeedTestResult(
        latency_ms=result.latency_ms,
        download_speed_mb_s=result.download_speed_mb_s,
        tested_at=tested_at,
    )
