from __future__ import annotations

from datetime import datetime
from typing import Iterable, Sequence

from sqlmodel import Session, select

from ..models import Node
from ..schemas import NodeCreate, NodeUpdate


def list_nodes(session: Session) -> Sequence[Node]:
    return session.exec(select(Node).order_by(Node.id)).all()


def get_node(session: Session, node_id: int) -> Node | None:
    return session.get(Node, node_id)


def create_node(session: Session, data: NodeCreate) -> Node:
    node = Node(**data.dict())
    session.add(node)
    session.commit()
    session.refresh(node)
    return node


def update_node(session: Session, node: Node, data: NodeUpdate) -> Node:
    payload = data.dict(exclude_unset=True)
    for key, value in payload.items():
        setattr(node, key, value)
    node.updated_at = datetime.utcnow()
    session.add(node)
    session.commit()
    session.refresh(node)
    return node


def delete_node(session: Session, node: Node) -> None:
    session.delete(node)
    session.commit()


def record_usage(session: Session, node: Node, upstream: int, downstream: int) -> Node:
    node.upstream_traffic = upstream
    node.downstream_traffic = downstream
    node.updated_at = datetime.utcnow()
    session.add(node)
    session.commit()
    session.refresh(node)
    return node


def bulk_update_latency(session: Session, results: Iterable[tuple[int, float | None, float | None]]) -> None:
    now = datetime.utcnow()
    for node_id, latency, speed in results:
        node = session.get(Node, node_id)
        if not node:
            continue
        if latency is not None:
            node.last_latency_ms = latency
        if speed is not None:
            node.last_speed_mb_s = speed
        node.updated_at = now
        session.add(node)
    session.commit()
