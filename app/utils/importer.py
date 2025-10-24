from __future__ import annotations

import base64
import json
from datetime import datetime
from typing import Any, Dict

import yaml

from ..models import ConfigFormat


def _maybe_base64(content: str) -> str:
    stripped = content.strip()
    try:
        padding = len(stripped) % 4
        if padding:
            stripped += "=" * (4 - padding)
        decoded = base64.b64decode(stripped, validate=True)
        return decoded.decode("utf-8")
    except Exception:
        return content


def _load_json(content: str) -> Dict[str, Any]:
    return json.loads(content)


def _load_yaml(content: str) -> Dict[str, Any]:
    return yaml.safe_load(content) or {}


def normalize_config(format_: ConfigFormat, raw_content: str) -> Dict[str, Any]:
    prepared = raw_content
    if format_ == ConfigFormat.V2RAYN:
        prepared = _maybe_base64(raw_content)
        try:
            data = json.loads(prepared)
        except json.JSONDecodeError:
            data = {"raw": prepared}
        return _normalize_generic(data)

    if format_ in {ConfigFormat.XRAY_JSON, ConfigFormat.SING_BOX_JSON}:
        data = _load_json(prepared)
        return _normalize_generic(data)

    if format_ == ConfigFormat.CLASH:
        data = _load_yaml(prepared)
        return _normalize_clash(data)

    if format_ == ConfigFormat.RAW:
        return {"raw": raw_content}

    raise ValueError(f"Unsupported config format: {format_}")


def _normalize_generic(data: Dict[str, Any]) -> Dict[str, Any]:
    result: Dict[str, Any] = {
        "inbounds": data.get("inbounds", []),
        "outbounds": data.get("outbounds", []),
        "routing": data.get("routing", {}),
        "dns": data.get("dns", {}),
        "metadata": {
            "remark": data.get("remark"),
            "generated_at": datetime.utcnow().isoformat(),
        },
    }
    if "outbounds" in data:
        result["metadata"]["outbound_count"] = len(data["outbounds"])
    if "routing" in data and isinstance(data["routing"], dict):
        result["metadata"]["rule_count"] = len(data["routing"].get("rules", []))
    return result


def _normalize_clash(data: Dict[str, Any]) -> Dict[str, Any]:
    proxies = data.get("proxies", [])
    proxy_groups = data.get("proxy-groups", [])
    rules = data.get("rules", [])
    return {
        "proxies": proxies,
        "proxy_groups": proxy_groups,
        "rules": rules,
        "metadata": {
            "proxy_count": len(proxies),
            "group_count": len(proxy_groups),
            "rule_count": len(rules),
            "generated_at": datetime.utcnow().isoformat(),
        },
    }
