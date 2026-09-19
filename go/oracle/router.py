"""Needle router parser and expansion fixtures."""

from __future__ import annotations

from typing import Any

from memento.router import (  # type: ignore[import-untyped]
    expand_router_action,
    parse_needle_router_output,
)


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
    expansions = []
    for item, request in [
        (cases[0], "Please find Piclaw"),
        (cases[1], "search for Piclaw"),
        (cases[2], "Kindly locate Piclaw"),
        (cases[3], "get Piclaw"),
        (cases[4], "status request"),
        (cases[5], "status request"),
        (cases[6], "fetch Piclaw"),
        (cases[7], "show Piclaw"),
        (cases[8], "show /a.md contents"),
        (cases[8], "show something else"),
        (cases[9], "book a flight"),
        (
            '[{"name":"search_then_read","arguments":{"query":"ignored","search_mode":"semantic"}}]',
            '"}; $doc.path; ${evil}; ../../etc/passwd',
        ),
    ]:
        action = parse_needle_router_output(item)
        expansion = expand_router_action(action, request=request)
        expansions.append(
            {
                "input": item,
                "request": request,
                "expansion": (expansion.model_dump(mode="json") if expansion is not None else None),
            }
        )
    return {"parsed": parsed, "errors": errors, "expansions": expansions}
