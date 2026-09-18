"""Capture python-frontmatter parsing and ruamel output without mutating the source."""

from __future__ import annotations

import importlib
import importlib.metadata
import json
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.repository.frontmatter")
    schema: Any = importlib.import_module("memento.repository.schema")
    base = {
        "id": "synthetic-id",
        "type": "concept",
        "title": "Synthetic",
        "status": "active",
        "created_at": "2026-09-18T12:34:56.123456Z",
        "updated_at": "2026-09-18T14:34:56+02:00",
        "updated_by": "test-agent",
    }
    serialized = []
    inputs: list[dict[str, Any]] = [base, {k: v for k, v in base.items() if k != "status"}]
    for key in ["id", "title", "description", "aliases", "tags"]:
        for text in [
            "",
            "simple",
            "a\nb",
            "a\n\nb\n",
            "a: b",
            "yes",
            "on",
            "null",
            "true",
            "2026-01-01",
            "1",
            "1.0",
            " leading",
            "trailing ",
            "a\tb",
            "\x00",
            "😀",
            "[]",
            "one #two",
            "---",
            "both ' and \"",
            "a\x85b",
            "\u00a0",
            "é\u2028b",
            "-",
            "?",
            ":",
            ":foo",
            "?foo",
            "-foo",
            "@foo",
            "a:b",
            "a#b",
            "x, y",
            ".nan",
            "1_000",
            "0x10",
            "y",
            "n",
            "nullify",
            "a\nb\u00a0",
            "a\u0085 b",
            "a \u2028b",
            "a\u0085\u0085b",
            "a\u2028",
            "x\ufeff",
            "x\x7f",
            "a: 'quoted'",
            "a" * 4200,
            "word " * 900,
        ]:
            inputs.append({**base, key: [text] if key in {"aliases", "tags"} else text})
    inputs.append({**base, "aliases": ["z", "a", "", "null", "a"]})
    for metadata in inputs:
        case: dict[str, Any] = {"metadata": metadata, "body": "\n  body  \r\n\n"}
        try:
            model = schema.ConceptFrontmatter.model_validate(metadata)
            case["expected"] = module.serialize_concept(
                schema.ConceptDocument(frontmatter=model, body=case["body"])
            )
        except Exception as exc:
            case["error"] = (
                "validation" if type(exc).__name__ == "ValidationError" else "serialization"
            )
        serialized.append(case)
    yaml = "\n".join(f"{k}: {json.dumps(v)}" for k, v in base.items())
    documents = [
        "",
        "plain",
        "---\n",
        "---\nid: x\n",
        "---\n[1,2]\n---\nbody",
        "---\n: bad\n---",
        "---\nnull\n---",
        "---\n" + yaml + "\n---\n body \n",
        "----\n" + yaml + "\n----\n body \n",
        " \n---\n" + yaml + "\n---\n body \n",
        "---\r\n" + yaml.replace("\n", "\r\n") + "\r\n---\r\n body \r\n",
        "---\n" + yaml + "\n...\nbody",
        "---\n" + yaml + "\n---\n  indented\nnext  ",
        "--- \t\n" + yaml + "\n---\nbody",
        "\ufeff---\n" + yaml + "\n---\nbody",
        "{\n" + json.dumps(base)[1:-1] + "\n}\n body \n",
        json.dumps(base) + "\nbody",
        "{\ninvalid\n}\nbody",
    ]
    for key, value in [
        ("title", "yes"),
        ("title", "on"),
        ("title", "012"),
        ("title", "null"),
        ("title", "!!str yes"),
        ("aliases", "[yes, no]"),
        ("created_at", "2026-09-18"),
        ("created_at", "2026-09-18T01:00:00"),
        ("created_at", "2026-09-18T01:00:00Z"),
        ("created_at", "0"),
        ("description", "!!binary YQ=="),
        ("description", "!!python/object {}"),
        ("description", "[unclosed"),
        ("description", "|\n  first\n  second"),
        ("description", ">\n  first\n  second"),
    ]:
        documents.append("---\n" + yaml + "\n" + key + ": " + value + "\n---\nbody")
    documents.extend(
        [
            "---\n" + yaml + "\ndescription: &v shared\ntitle: *v\n---\nbody",
            "---\n<<: {"
            + ", ".join(f"{k}: {json.dumps(v)}" for k, v in base.items())
            + "}\n---\nbody",
            "---\n" + yaml + "\ndescription: &loop [*loop]\n---\nbody",
        ]
    )
    for key in ["description", "schema_version", "created_at", "title"]:
        for value in [
            "08",
            "0o10",
            "0b10",
            "0x10",
            "-012",
            "1e3",
            "1.0e3",
            "1.0e+3",
            "1:02",
            "-1:02",
            "1:02.5",
            "true",
            "!!bool true",
            "!!bool bad",
            "!!int bad",
            "!!int 0o10",
            ".inf",
            "-.inf",
            ".nan",
            "!!float bad",
            "!!timestamp bad",
            "!!binary @@",
            "!!binary /w==",
            "!custom value",
        ]:
            documents.append("---\n" + yaml + "\n" + key + ": " + value + "\n---\nbody")
    for extra in [
        "<<: [{description: first}, {description: second}]",
        "<<: 1",
        "<<: !bad []",
        "? [a, b]\n: value",
        "1: value",
        "!bad key: value",
        "description: [!bad value]",
        "description: &v {x: *v}",
        "description: !!set {x: null}",
    ]:
        documents.append("---\n" + yaml + "\n" + extra + "\n---\nbody")
    documents.extend(["---\n---\nbody", "{\n", "{\n}\nbody", '{\n"a": 1\n}\nbody'])
    parsed = []
    for text in documents:
        item: dict[str, Any] = {"input": text}
        try:
            model = module.parse_concept_text(text)
            item["expected"] = model.model_dump(mode="json")
        except module.FrontmatterError as exc:
            item["error"] = str(exc)
        parsed.append(item)
    return {
        "libraries": {
            name: importlib.metadata.version(name)
            for name in ["pydantic", "pydantic-core", "PyYAML", "ruamel.yaml", "python-frontmatter"]
        },
        "serialize": serialized,
        "parse": parsed,
    }
