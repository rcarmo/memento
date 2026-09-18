"""Capture Markdown-it structural links and parser-confirmed rename edits."""

from __future__ import annotations

import importlib
import importlib.metadata
from dataclasses import asdict
from typing import Any


def fixtures() -> dict[str, Any]:
    module: Any = importlib.import_module("memento.repository.links")
    documents = [
        "",
        "[one](/old.md)",
        "# [one](/old.md)\n\n[second](/old.md#anchor)",
        "prose /old.md and `[code](/old.md)`\n\n```md\n[x](/old.md)\n```\n",
        "[a **bold** and *em* `code` ![alt](/image.png)](/old.md)",
        "[a\\*b &amp; &#65; &unknown;](/old.md)",
        "[one\ntwo](/old.md)\n\n[one  \ntwo](/old.md)",
        "> [quote](/old.md)\n>\n> next\n\n- [list](/old.md)\n  continued\n\n  [nested](/old.md)",
        "    [indented](/old.md)\n\n[normal](/old.md)",
        "![image](/old.md) [![nested](/old.md)](/other.md)",
        '[reference][key]\n\n[key]: /old.md "Title"\n',
        "[key][] [key]\n\n[key]: /old.md\n",
        "[unused]: /old.md\n",
        "[x](/old.md?q=1) [x](/old.md2) [x](relative/old.md)",
        '[title](/other.md "/old.md")',
        "[x](</old.md>)",
        "[x](</old.md#space here>)",
        "[x](/old\\.md) [x](/old%2Emd)",
        "<https://example.com/old.md> <person@example.com>",
        "<https://éxample.com/old.md> <mailto:a@éxample.com>",
        "[x](https://éxample.com/old.md?q=one#two)",
        '<a href="/old.md">HTML</a> [x](/old.md)\n',
        "[<b>bold</b> text](/old.md)",
        "[x](javascript:alert) ![x](file:///old.md)",
        "[x](data:text/plain,x) [x](data:image/png;base64,AA==)",
        "[a [nested](/old.md)](/outer.md)",
        "[x](/old.md \"title\") [y](/old.md 'title')",
        "[x](//example.com/old.md) [x](HTTP://EXAMPLE.COM/old.md)",
        "[x](/café.md) [x](/space%20here.md) [x](/a?b=1&amp;c=2)",
        "[x](</a b.md>) [x](</a%zz.md>) [x](/a\\(b\\).md)",
        "line\r\n[x](/old.md)\r\n\r\n## [h](/old.md)",
        "[x](vbscript:bad) [x](FILE:x) [x](custom:value)",
        "[x](<javascript:alert>) [x](&#106;avascript:alert)",
        "[text\\\nnext](/old.md)",
        "[<https://example.com>](/old.md)",
        "# heading\nparagraph\n[x](/old.md)\n\n---\n\n[x](/old.md)",
        "a | [b](/old.md)\n--|--\nc | d",
        "[x](/old.md) [x](/old.md#one#two)",
    ]
    # Matrix expands escaping, nested formatting and destination normalization.
    for label in [
        "a",
        "*em*",
        "**bold**",
        "`code`",
        "a &amp; b",
        "a\\[b",
        "![alt](/image)",
        "a\nb",
        "a  \nb",
        "<i>html</i>",
    ]:
        for href in [
            "/old.md",
            "/old.md#a",
            "https://host/a",
            "//host/a",
            "javascript:x",
            "data:image/png;x",
            "/café",
        ]:
            documents.append(f"[{label}]({href})")
    documents.extend(
        [
            "<https://host/a%20b%2fc%2Fd%3Fq%3dq%25%zz>",
            "<https://[::1]:8080/old.md>",
            "[x](https://user:p@éxample.com:8080/old.md) [x](ftp://éxample.com/old.md)",
            "[x](https://EXAMPLE.com/old.md) [x](https://a_b.com/old.md)",
            "[one **nested *em*** ![alt](x)](/old.md)",
            "[x](<data:text/plain,foo>) [![alt](x)](javascript:x)",
            "![outer [link](/old.md)](/old.md)",
            "[bad [inner](/old.md)](javascript:x)",
            "[label **em**](javascript:x)",
            "[foo](/old.md)\\n[bar](/old.md)",
        ]
    )
    cases = []
    for text in documents:
        cases.append(
            {
                "input": text,
                "links": [asdict(link) for link in module.extract_structural_links(text)],
                "rename": asdict(
                    module.rewrite_links_for_rename(text, old_path="/old.md", new_path="/new.md")
                ),
            }
        )
    rewrites = []
    for content, old, new in [
        ("[x](/old.md)", "/old.md", "[bad](/new.md)"),
        ("[x](/old.md)", "/old.md", "/new path"),
        ("[x](/old.md)", "/old.md", "/new.md#x"),
        ("[x](relative.md)", "relative.md", "new.md"),
        ("[x](/old.md)", "/old.md", "/café.md"),
        ("[x](/old.md)", "/old.md", "/a(b).md"),
    ]:
        rewrites.append(
            {
                "input": content,
                "old": old,
                "new": new,
                "expected": asdict(
                    module.rewrite_links_for_rename(content, old_path=old, new_path=new)
                ),
            }
        )
    return {
        "rewrites": rewrites,
        "markdown-it-py": importlib.metadata.version("markdown-it-py"),
        "cases": cases,
        "external": [
            {"input": href, "expected": module.is_external_link(href)}
            for href in [
                "",
                "/a",
                "//a",
                "///a",
                "a:b",
                "a+b.c-d:x",
                "1a:x",
                "ä:x",
                " mailto:x",
                "mailto:x",
                "#x",
                "a/b:c",
            ]
        ],
    }
