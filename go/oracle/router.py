"""Needle router parser fixtures."""

from __future__ import annotations

from typing import Any

from memento.router import parse_needle_router_output  # type: ignore[import-untyped]


def fixtures() -> dict[str, Any]:
    cases = [
        '[{"name":"search_then_read","arguments":{"query":"Piclaw"}}]',
        '[{"name":"search_then_read","arguments":{"query":"Piclaw","search_mode":"semantic"}}]',
        '[{"name":"search_paths","arguments":{"query":"Piclaw"}}]',
        '[{"name":"search_paths","arguments":{"query":"Piclaw","limit":5,"search_mode":"hybrid"}}]',
        '[{"name":"status_field","arguments":{"field":"indexed"}}]',
        '[{"name":"status_field","arguments":{"field":"status"}}]',
        '[{"name":"search_then_graph","arguments":{"query":"Piclaw"}}]',
        '[{"name":"search_then_graph","arguments":{"query":"Piclaw","depth":2,"search_mode":"semantic"}}]',
        '[{"name":"read_field","arguments":{"id_or_path":"/a.md","field":"contents"}}]',
        '[{"name":"UNKNOWN","arguments":{}}]',
    ]
    parsed = [
        {
            "input": item,
            "action": parse_needle_router_output(item).model_dump(mode="json"),
        }
        for item in cases
    ]
    errors = []
    for item in [
        "bad",
        "[]",
        "[{}]",
        "[1]",
        '[{"name":""}]',
        '[{"name":"UNKNOWN","arguments":[]}]',
        '[{"name":"UNKNOWN","arguments":{},"extra":1}]',
        '[{"name":"search_paths","arguments":{"query":"Piclaw","limit":4}}]',
    ]:
        try:
            parse_needle_router_output(item)
        except Exception as exc:
            errors.append({"input": item, "type": type(exc).__name__})
    return {"parsed": parsed, "errors": errors}
