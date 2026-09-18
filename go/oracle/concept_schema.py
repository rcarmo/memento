"""Capture field coercion, UTC handling and body normalisation from the baseline."""

from __future__ import annotations

import importlib
from typing import Any


def fixtures() -> dict[str, Any]:
    schema: Any = importlib.import_module("memento.repository.schema")
    frontmatter: Any = importlib.import_module("memento.repository.frontmatter")
    base = {
        "id": "synthetic-id",
        "type": "concept",
        "title": "Synthetic",
        "created_at": "2026-09-18T12:34:56.123456Z",
        "updated_at": "2026-09-18T14:34:56+02:00",
        "updated_by": "test-agent",
    }
    inputs: list[Any] = [base, None, [], "", {}, {**base, "unknown": 1}]
    for name in base:
        inputs.append({key: value for key, value in base.items() if key != name})
    values: list[Any] = [
        None,
        True,
        False,
        0,
        1,
        1.0,
        -1,
        1.5,
        "",
        " ",
        "one",
        [],
        {},
        ["one"],
        [1],
        ["a", "", "a", "Z", "ß", "é", "é", " b "],
    ]
    for name in [
        "schema_version",
        "id",
        "type",
        "title",
        "status",
        "description",
        "aliases",
        "tags",
        "source_refs",
        "supersedes",
        "updated_by",
        "created_at",
        "updated_at",
    ]:
        inputs.extend({**base, name: value} for value in values)
    for name in ["title", "id", "description", "updated_by", "aliases"]:
        for text in [
            "line\nnext",
            "tab\there",
            "\x00",
            "\x1f",
            "\x7f",
            "\x85",
            "é",
            "😀",
            "<script>",
            " a ",
        ]:
            inputs.append({**base, name: [text] if name == "aliases" else text})
    for value in ["concept", "instance", "person", "project", "service", "system", "Concept"]:
        inputs.append({**base, "type": value})
    for value in ["active", "deprecated", "tombstone", "ACTIVE"]:
        inputs.append({**base, "status": value})
    for value in [
        "1",
        "01",
        "+1",
        " 1 ",
        "1.0",
        "01.00",
        "1e0",
        "1_0",
        "0_1",
        "1.",
        "١",
        "１",
        "1\x85",
        "1\x1c",
        "1\x00",
    ]:
        inputs.append({**base, "schema_version": value})
    timestamps: list[Any] = [
        "2026-09-18",
        "2026-09-18T12:34:56",
        "2026-09-18T12:34Z",
        "2026-09-18t12:34:56z",
        "2026-09-18 12:34:56+0130",
        "2026-09-18T12:34:56+01",
        "2026-09-18T12:34:56+24:00",
        "2026-09-18T12:34:56+00:60",
        "2026-09-18T12:34:56-01:30",
        "2026-09-18T12:34:56.123456789Z",
        "2026-09-18T12:34:56,1234567Z",
        "2026-09-18T24:00:00Z",
        "2026-02-29T00:00:00Z",
        "2024-02-29T00:00:00Z",
        "2026-09-18X12:34:56Z",
        "2026-09-18T12:34:60Z",
        "0001-01-01T00:00:00Z",
        "9999-12-31T23:59:59.999999Z",
        "0",
        "-1",
        "+1",
        "1.5",
        "-1.5",
        " 0 ",
        "1e3",
        "NaN",
        "20000000000",
        "20000000001",
        "-20000000000",
        "-20000000001",
        "2026-09-18T12:34:56Z ",
        0,
        1,
        -1,
        1.000001,
        1.1234567,
        -1.1234567,
        -0.1,
        -1.9,
        -0.9999999,
        -1.0000005,
        1.9999999,
        20000000001.5,
        -20000000001.5,
        "-1.1",
        "-1.9",
        "20000000000.5",
        20000000000,
        20000000001,
        -20000000000,
        -20000000001,
        20000000000.5,
        -20000000000.5,
        253402300799000,
        253402300800000,
        -62135596800000,
        -62135596800001,
    ]
    inputs.extend({**base, "created_at": value} for value in timestamps)
    cases = []
    for value in inputs:
        try:
            model = schema.ConceptFrontmatter.model_validate(value)
            expected = model.model_dump(mode="json")
        except Exception:
            expected = None
        cases.append({"input": value, "expected": expected})
    bodies = [
        "",
        "\n",
        "  hello  \r\n world\t\r",
        "\n\n# Title\n\n",
        "  \n text \n\t",
        "a\x85\n\x1c\nb\u00a0",
        "\r\n\r\nx\r\n",
        "  leading space",
        "\tindented\n\nend   ",
    ]
    return {
        "metadata": cases,
        "bodies": [
            {"input": body, "expected": frontmatter.normalize_concept_body(body)} for body in bodies
        ],
    }
