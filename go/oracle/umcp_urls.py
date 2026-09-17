"""URL/Origin edge cases from the actual shared reference and urllib parsing."""

from __future__ import annotations

from typing import Any


def fixtures(shared: Any) -> dict[str, Any]:
    origins = [
        "",
        "http://localhost",
        "HTTP://LOCALHOST",
        "https://localhost",
        "http://127.0.0.1",
        "http://[::1]",
        "http://[::1]:123",
        "http://localhost/",
        "http://localhost?",
        "http://localhost#",
        "http://localhost?x",
        "http://localhost#x",
        "http://localhost:",
        "http://localhost:0",
        "http://localhost:000",
        "http://localhost:65535",
        "http://localhost:65536",
        "http://localhost:bad",
        "http://localhost:٠",
        "http://localhost:99999999999999999999999999",
        "http://localhost\t",
        "http://user@localhost",
        "http://:pass@localhost",
        "http://",
        "http://example.com",
        "https://ui.example",
        "ftp://localhost",
        "//localhost",
        "http://[bad]",
        "http://[127.0.0.1]",
        "http://[v1.test]",
        "http://[vbad]",
        "http://[::1",
        "http://::1]",
        "http://x[::1]",
        "http://[::1]x",
        "http://[::1]:bad",
        "http://localhost／bad",
        "http://localhost℀bad",
        "http://[fe80::1%zone]",
        "http://[::1]::1",
        "http://[b'ad]",
        'http://[b"ad]',
        "http://[b'\"ad]",
        "http://[b\\ad]",
        "http://[b\x01ad]",
    ]
    origin_cases = []
    for value in origins:
        for allowed, local, authority in [
            ([], True, None),
            ([], False, None),
            (["https://ui.example"], False, None),
            ([], False, "example.com"),
            ([value], False, None),
        ]:
            try:
                valid = shared.origin_is_allowed(
                    value, allowed, local_bind=local, request_authority=authority
                )
                error = None
            except ValueError as exc:
                valid = False
                error = str(exc)
            origin_cases.append(
                {
                    "origin": value,
                    "allowed": allowed,
                    "local": local,
                    "authority": authority,
                    "valid": valid,
                    "error": error,
                }
            )
    targets = [
        "",
        "/",
        "*",
        "?x=1",
        "#x",
        "//host/path",
        "/bad%xx",
        " /ok",
        "\x00/path",
        "http://host",
        "http://host/a;b?x#frag",
        "mailto:abc",
        "1bad:foo",
        "HTTP://HOST/a%20b",
        "http://[bad]/a",
        "http://localhost／bad/a",
        "a\tb\nc\rd",
    ]
    paths = []
    for value in targets:
        try:
            split = shared.split_request_target(value)
            result = {
                "scheme": split.scheme,
                "netloc": split.netloc,
                "path": split.path,
                "query": split.query,
                "fragment": split.fragment,
                "request_path": shared.request_target_path(value),
            }
            error = None
        except ValueError as exc:
            result = None
            error = str(exc)
        paths.append({"input": value, "expected": result, "error": error})
    return {"origins": origin_cases, "targets": paths}
