from __future__ import annotations

import asyncio
import statistics
import time
from dataclasses import dataclass
from typing import Optional

import httpx


@dataclass
class LatencyResult:
    latency_ms: Optional[float]


@dataclass
class SpeedResult:
    latency_ms: Optional[float]
    download_speed_mb_s: Optional[float]


async def measure_latency(host: str, port: int, attempts: int = 3, timeout: float = 5.0) -> LatencyResult:
    latencies: list[float] = []
    for _ in range(attempts):
        start = time.perf_counter()
        try:
            reader, writer = await asyncio.wait_for(asyncio.open_connection(host, port), timeout)
        except Exception:
            continue
        else:
            elapsed = (time.perf_counter() - start) * 1000.0
            latencies.append(elapsed)
            writer.close()
            await writer.wait_closed()
        finally:
            await asyncio.sleep(0.1)
    if not latencies:
        return LatencyResult(latency_ms=None)
    return LatencyResult(latency_ms=statistics.fmean(latencies))


async def measure_speed(
    host: str,
    port: int,
    test_url: str,
    download_bytes: int = 1_000_000,
    proxy: Optional[str] = None,
    timeout: float = 20.0,
) -> SpeedResult:
    latency_result = await measure_latency(host, port)
    proxies = None
    if proxy:
        proxies = {
            "http": proxy,
            "https": proxy,
        }
    total_bytes = 0
    start = time.perf_counter()
    try:
        async with httpx.AsyncClient(proxies=proxies, timeout=timeout) as client:
            async with client.stream("GET", test_url) as response:
                response.raise_for_status()
                async for chunk in response.aiter_bytes():
                    total_bytes += len(chunk)
                    if total_bytes >= download_bytes:
                        break
    except Exception:
        return SpeedResult(latency_ms=latency_result.latency_ms, download_speed_mb_s=None)
    duration = time.perf_counter() - start
    if duration <= 0 or total_bytes == 0:
        return SpeedResult(latency_ms=latency_result.latency_ms, download_speed_mb_s=None)
    megabytes = total_bytes / (1024 * 1024)
    speed_mb_s = megabytes / duration
    return SpeedResult(latency_ms=latency_result.latency_ms, download_speed_mb_s=speed_mb_s)
