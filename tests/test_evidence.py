from __future__ import annotations

from datetime import UTC, datetime

import pytest
from pydantic import ValidationError

from memento.evidence import (
    EvidenceItem,
    EvidenceRank,
    QueryProfile,
    evidence_is_sufficient,
    is_currently_ineligible,
    is_sensitive_evidence,
    namespace_matches,
    profile_question,
)


@pytest.mark.parametrize(
    ("question", "secret", "temporal", "relational", "namespace"),
    [
        ("What is the API key?", True, "neutral", False, None),
        ("What is current in my personal memory?", False, "current", False, "personal"),
        ("What was used before migration?", False, "historical", False, None),
        ("Which rack is Atlas connected to?", False, "neutral", True, None),
        ("What is in the work workspace?", False, "neutral", False, "work"),
    ],
)
def test_profile_question_classifies_policy_relevant_intent(
    question: str,
    secret: bool,
    temporal: str,
    relational: bool,
    namespace: str | None,
) -> None:
    profile = profile_question(question)
    assert profile.secret_intent is secret
    assert profile.temporal_intent == temporal
    assert profile.relational is relational
    assert profile.namespace_hint == namespace


def test_profile_does_not_treat_my_or_token_usage_as_secret_namespace_intent() -> None:
    profile = profile_question("Show my token usage trend")
    assert profile.secret_intent is False
    assert profile.namespace_hint is None


def test_namespace_matching_is_strict_for_explicit_namespace_queries() -> None:
    personal = profile_question("my personal Atlas")
    assert namespace_matches(personal, path="/personal/atlas.md") is True
    assert namespace_matches(personal, path="/work/atlas.md") is False
    assert namespace_matches(personal, path="/public/atlas.md") is False

    work = profile_question("work Atlas")
    assert namespace_matches(work, path="/work/atlas.md") is True
    assert namespace_matches(work, path="/personal/atlas.md") is False
    assert namespace_matches(work, path="/public/atlas.md") is False


def test_sensitive_current_and_historical_filters_are_deterministic() -> None:
    assert is_sensitive_evidence(tags=("shared", "secret")) is True
    assert is_sensitive_evidence(tags=("shared",)) is False
    assert is_currently_ineligible(status="deprecated", tags=()) is True
    assert is_currently_ineligible(status="active", tags=("stale",)) is True
    assert is_currently_ineligible(status="active", tags=("accepted",)) is False


def test_evidence_sufficiency_requires_query_namespace_and_temporal_support() -> None:
    current = profile_question("What is the current personal Atlas port?")
    assert evidence_is_sufficient(
        current,
        texts=(("Personal Atlas", "Current Atlas port is 8443."),),
        paths=("/personal/atlas.md",),
        statuses_and_tags=(("active", ("personal",)),),
    )
    assert not evidence_is_sufficient(
        current,
        texts=(("Work Atlas", "Current Atlas port is 9443."),),
        paths=("/work/atlas.md",),
        statuses_and_tags=(("active", ("work",)),),
    )

    historical = profile_question("What was Atlas before migration?")
    assert evidence_is_sufficient(
        historical,
        texts=(("Legacy Atlas", "Atlas before migration."),),
        paths=("/work/atlas-old.md",),
        statuses_and_tags=(("deprecated", ("historical",)),),
    )
    assert not evidence_is_sufficient(
        historical,
        texts=(("Atlas", "Atlas migration."),),
        paths=("/work/atlas.md",),
        statuses_and_tags=(("active", ()),),
    )


def test_evidence_item_requires_explicit_untrusted_marker_and_provenance() -> None:
    payload = {
        "id": "atlas-id",
        "path": "/work/atlas.md",
        "title": "Atlas",
        "revision": "a" * 40,
        "status": "active",
        "updated_at": datetime(2026, 8, 7, tzinfo=UTC),
        "retrieval_reasons": ("hybrid_primary",),
        "ranks": (EvidenceRank(source="hybrid", rank=1, score=0.9),),
        "support_chain": ("atlas-id",),
        "authorization_scope": "scope-hash",
    }
    item = EvidenceItem.model_validate(payload)
    assert item.untrusted is True
    assert item.ranks[0].rank == 1
    assert item.authorization_scope == "scope-hash"

    with pytest.raises(ValidationError):
        EvidenceItem.model_validate({**payload, "untrusted": False})


def test_query_profile_is_closed_to_unknown_contract_fields() -> None:
    with pytest.raises(ValidationError):
        QueryProfile.model_validate(
            {
                "secret_intent": False,
                "temporal_intent": "neutral",
                "relational": False,
                "terms": [],
                "unexpected": True,
            }
        )
