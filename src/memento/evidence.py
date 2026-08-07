from __future__ import annotations

import re
from collections.abc import Sequence
from datetime import datetime
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

TemporalIntent = Literal["current", "historical", "neutral"]
NamespaceHint = Literal["personal", "work"]
EvidenceSource = Literal["hybrid", "lexical_fallback", "graph", "hot_memory"]
GraphDirection = Literal["outbound", "inbound"]

_SECRET_PHRASES = (
    "api key",
    "credential",
    "credentials",
    "password",
    "passphrase",
    "private key",
    "recovery code",
    "rotation code",
    "secret",
    "access token",
    "api token",
    "auth token",
    "bearer token",
    "token value",
)
_HISTORICAL_PHRASES = (
    "before ",
    "formerly",
    "historical",
    "history",
    "previously",
    "prior to",
    "used to",
    "why was",
)
_CURRENT_TERMS = frozenset(
    {"accepted", "current", "currently", "latest", "now", "recent", "recently", "today"}
)
_RELATIONAL_PHRASES = (
    "connected to",
    "connected with",
    "depend on",
    "depends on",
    "linked to",
    "managed by",
    "manages",
    "owned by",
    "owns",
    "powered by",
    "powers",
    "rack that",
    "related to",
    "reports to",
    "which host",
    "which rack",
    "which service",
    "who manages",
    "who owns",
)
_QUERY_STOP_WORDS = frozenset(
    {
        "a",
        "about",
        "an",
        "and",
        "are",
        "at",
        "be",
        "by",
        "can",
        "did",
        "do",
        "does",
        "for",
        "from",
        "how",
        "in",
        "is",
        "it",
        "me",
        "of",
        "on",
        "please",
        "should",
        "tell",
        "the",
        "this",
        "to",
        "was",
        "were",
        "what",
        "when",
        "where",
        "which",
        "who",
        "why",
        "will",
        "with",
        "would",
    }
)


class QueryProfile(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    secret_intent: bool
    temporal_intent: TemporalIntent
    relational: bool
    namespace_hint: NamespaceHint | None = None
    terms: tuple[str, ...] = ()


class EvidenceRank(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    source: EvidenceSource
    rank: int = Field(ge=1)
    score: float | None = None


class EvidenceItem(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    id: str
    path: str
    title: str
    revision: str
    status: str
    updated_at: datetime
    tags: tuple[str, ...] = ()
    source_refs: tuple[str, ...] = ()
    supersedes: tuple[str, ...] = ()
    retrieval_reasons: tuple[str, ...]
    ranks: tuple[EvidenceRank, ...]
    support_chain: tuple[str, ...]
    graph_anchor_id: str | None = None
    graph_anchor_path: str | None = None
    graph_depth: int | None = Field(default=None, ge=1)
    graph_direction: GraphDirection | None = None
    authorization_scope: str
    untrusted: Literal[True] = True


class EvidenceSet(BaseModel):
    model_config = ConfigDict(extra="forbid", frozen=True)

    schema_version: Literal[1] = 1
    query_profile: QueryProfile
    authorization_scope: str
    retrieval_strategy: str
    escalated: bool = False
    sufficient: bool = False
    abstention_reason: str | None = None
    items: tuple[EvidenceItem, ...] = ()


def profile_question(question: str) -> QueryProfile:
    normalized = " ".join(question.casefold().split())
    words = tuple(re.findall(r"\w+", normalized, flags=re.UNICODE))
    terms = tuple(dict.fromkeys(word for word in words if word not in _QUERY_STOP_WORDS))
    historical = any(phrase in normalized for phrase in _HISTORICAL_PHRASES)
    temporal_intent: TemporalIntent
    if historical:
        temporal_intent = "historical"
    elif _CURRENT_TERMS.intersection(words):
        temporal_intent = "current"
    else:
        temporal_intent = "neutral"
    namespace_hint: NamespaceHint | None = None
    if "personal" in words:
        namespace_hint = "personal"
    elif "work" in words or "workspace" in words or "company" in words:
        namespace_hint = "work"
    return QueryProfile(
        secret_intent=any(phrase in normalized for phrase in _SECRET_PHRASES),
        temporal_intent=temporal_intent,
        relational=any(phrase in normalized for phrase in _RELATIONAL_PHRASES),
        namespace_hint=namespace_hint,
        terms=terms,
    )


def namespace_matches(profile: QueryProfile, *, path: str) -> bool:
    if profile.namespace_hint == "personal":
        return path.startswith("/personal/")
    if profile.namespace_hint == "work":
        return path.startswith("/work/")
    return True


def is_sensitive_evidence(*, tags: tuple[str, ...]) -> bool:
    normalized = {tag.casefold() for tag in tags}
    return bool(normalized.intersection({"credential", "credentials", "secret", "secrets"}))


def is_currently_ineligible(*, status: str, tags: tuple[str, ...]) -> bool:
    normalized = {tag.casefold() for tag in tags}
    return status in {"deprecated", "tombstone"} or bool(
        normalized.intersection({"conflicting", "historical", "obsolete", "stale"})
    )


def evidence_text_matches(profile: QueryProfile, *, title: str, body: str) -> bool:
    if not profile.terms:
        return False
    text_terms = set(re.findall(r"\w+", f"{title} {body}".casefold(), flags=re.UNICODE))
    matched = 0
    for term in profile.terms:
        if term in text_terms or (
            len(term) >= 5 and any(word.startswith(term[:5]) for word in text_terms)
        ):
            matched += 1
    required = 1 if len(profile.terms) == 1 else 2
    return matched >= required


def evidence_is_sufficient(
    profile: QueryProfile,
    *,
    texts: Sequence[tuple[str, str]],
    paths: Sequence[str],
    statuses_and_tags: Sequence[tuple[str, tuple[str, ...]]],
) -> bool:
    if not texts or len(texts) != len(paths) or len(texts) != len(statuses_and_tags):
        return False
    if not any(evidence_text_matches(profile, title=title, body=body) for title, body in texts):
        return False
    if profile.namespace_hint is not None:
        namespace_prefix = f"/{profile.namespace_hint}/"
        if not any(path.startswith(namespace_prefix) for path in paths):
            return False
    if profile.temporal_intent == "historical":
        return any(
            status == "deprecated" or bool({"historical", "temporal"}.intersection(tags))
            for status, tags in statuses_and_tags
        )
    if profile.temporal_intent == "current":
        return any(
            status == "active" and not is_currently_ineligible(status=status, tags=tags)
            for status, tags in statuses_and_tags
        )
    return True
