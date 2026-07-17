from __future__ import annotations

from dataclasses import dataclass

from markdown_it import MarkdownIt
from markdown_it.token import Token

_MARKDOWN = MarkdownIt("commonmark")


@dataclass(frozen=True, slots=True)
class MarkdownLink:
    href: str
    text: str
    line: int | None


@dataclass(frozen=True, slots=True)
class RenameRewriteResult:
    content: str
    changed: bool


class MarkdownLinkError(Exception):
    """Raised when Markdown link processing fails."""


def extract_structural_links(content: str) -> list[MarkdownLink]:
    tokens = _MARKDOWN.parse(content)
    links: list[MarkdownLink] = []
    for token in tokens:
        if token.type != "inline" or not token.children:
            continue
        links.extend(_extract_inline_links(token.children, token.map[0] + 1 if token.map else None))
    return links


def rewrite_links_for_rename(content: str, *, old_path: str, new_path: str) -> RenameRewriteResult:
    tokens = _MARKDOWN.parse(content)
    changed = False
    for token in _walk_tokens(tokens):
        if token.type not in {"link_open", "image"}:
            continue
        raw_href = token.attrGet("href") if token.type == "link_open" else token.attrGet("src")
        if not isinstance(raw_href, str):
            continue
        rewritten = _rewrite_href(raw_href, old_path=old_path, new_path=new_path)
        href = raw_href
        if rewritten == href:
            continue
        changed = True
        attr_name = "href" if token.type == "link_open" else "src"
        token.attrSet(attr_name, rewritten)
    if not changed:
        return RenameRewriteResult(content=content, changed=False)
    rendered = _MARKDOWN.renderer.render(tokens, _MARKDOWN.options, {})
    return RenameRewriteResult(content=rendered, changed=True)


def _extract_inline_links(children: list[Token], line: int | None) -> list[MarkdownLink]:
    links: list[MarkdownLink] = []
    current_href: str | None = None
    current_text: list[str] = []
    for child in children:
        if child.type == "link_open":
            href = child.attrGet("href")
            current_href = href if isinstance(href, str) else None
            current_text = []
        elif child.type == "text" and current_href is not None:
            current_text.append(child.content)
        elif child.type == "link_close" and current_href is not None:
            links.append(MarkdownLink(href=current_href, text="".join(current_text), line=line))
            current_href = None
            current_text = []
    return links


def _walk_tokens(tokens: list[Token]) -> list[Token]:
    walked: list[Token] = []
    for token in tokens:
        walked.append(token)
        if token.children:
            walked.extend(_walk_tokens(token.children))
    return walked


def _rewrite_href(href: str, *, old_path: str, new_path: str) -> str:
    if "#" in href:
        path, anchor = href.split("#", 1)
        rewritten_path = _rewrite_bundle_path(path, old_path=old_path, new_path=new_path)
        return f"{rewritten_path}#{anchor}" if rewritten_path != path else href
    return _rewrite_bundle_path(href, old_path=old_path, new_path=new_path)


def _rewrite_bundle_path(path: str, *, old_path: str, new_path: str) -> str:
    if not path.startswith("/"):
        return path
    if path != old_path:
        return path
    return new_path
