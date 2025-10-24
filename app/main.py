from __future__ import annotations

import asyncio
import logging
from contextlib import asynccontextmanager
from datetime import datetime

from fastapi import FastAPI
from sqlmodel import Session

from .database import engine, get_session, init_db
from .routers import configs, geo, nodes, traffic
from .services import config_service, geo_service, node_service, traffic_service
from .services.metrics import measure_latency

logger = logging.getLogger(__name__)


async def _wait_with_stop(event: asyncio.Event, timeout: float) -> None:
    try:
        await asyncio.wait_for(event.wait(), timeout=timeout)
    except asyncio.TimeoutError:
        return


async def _config_sync_loop(stop_event: asyncio.Event) -> None:
    while not stop_event.is_set():
        try:
            with Session(engine) as session:
                refreshed = config_service.refresh_due_profiles(session)
                if refreshed:
                    logger.info("Refreshed %d config profiles", len(refreshed))
        except Exception as exc:  # pragma: no cover - logging only
            logger.exception("Config sync loop error: %s", exc)
        await _wait_with_stop(stop_event, timeout=60)


async def _geo_sync_loop(stop_event: asyncio.Event) -> None:
    while not stop_event.is_set():
        try:
            with Session(engine) as session:
                refreshed = geo_service.refresh_due_resources(session)
                if refreshed:
                    logger.info("Refreshed %d geo resources", len(refreshed))
        except Exception as exc:  # pragma: no cover - logging only
            logger.exception("Geo sync loop error: %s", exc)
        await _wait_with_stop(stop_event, timeout=3600)


async def _telemetry_loop(stop_event: asyncio.Event) -> None:
    while not stop_event.is_set():
        interval = 600
        try:
            with Session(engine) as session:
                settings = traffic_service.get_settings(session)
                telemetry = settings.telemetry or {}
                interval = max(int(telemetry.get("latency_interval_minutes", 10)), 1) * 60
                if telemetry.get("auto_latency_test"):
                    nodes_to_probe = list(node_service.list_nodes(session))
                else:
                    nodes_to_probe = []
            if nodes_to_probe:
                results = []
                for node in nodes_to_probe:
                    result = await measure_latency(node.address, node.port)
                    results.append((node.id, result.latency_ms, None))
                with Session(engine) as session:
                    node_service.bulk_update_latency(session, results)
        except Exception as exc:  # pragma: no cover - logging only
            logger.exception("Telemetry loop error: %s", exc)
        await _wait_with_stop(stop_event, timeout=interval)


@asynccontextmanager
async def lifespan(app: FastAPI):
    init_db()
    stop_event = asyncio.Event()
    tasks = [
        asyncio.create_task(_config_sync_loop(stop_event)),
        asyncio.create_task(_geo_sync_loop(stop_event)),
        asyncio.create_task(_telemetry_loop(stop_event)),
    ]
    try:
        yield
    finally:
        stop_event.set()
        for task in tasks:
            task.cancel()
        await asyncio.gather(*tasks, return_exceptions=True)


app = FastAPI(title="Vea Control", version="0.1.0", lifespan=lifespan)

app.include_router(nodes.router)
app.include_router(configs.router)
app.include_router(geo.router)
app.include_router(traffic.router)


@app.get("/health")
def healthcheck() -> dict[str, str]:
    return {"status": "ok", "timestamp": datetime.utcnow().isoformat()}


__all__ = ["app", "get_session"]
