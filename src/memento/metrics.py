from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from memento.app import MementoRuntime


@dataclass(frozen=True, slots=True)
class MetricsSnapshot:
    service_up: int
    repo_revision: str
    index_revision: str
    index_stale: bool
    visible_concepts: int
    proposal_backlog: int
    control_db_open: bool

    def as_dict(self) -> dict[str, Any]:
        return {
            "service_up": self.service_up,
            "repo_revision": self.repo_revision,
            "index_revision": self.index_revision,
            "index_stale": self.index_stale,
            "visible_concepts": self.visible_concepts,
            "proposal_backlog": self.proposal_backlog,
            "control_db_open": self.control_db_open,
        }


def collect_metrics(runtime: MementoRuntime) -> MetricsSnapshot:
    status = runtime.status_snapshot()
    return MetricsSnapshot(
        service_up=1,
        repo_revision=status["repo_revision"],
        index_revision=status["index_revision"],
        index_stale=bool(status["index_stale"]),
        visible_concepts=int(status["visible_concepts"]),
        proposal_backlog=int(status["proposal_backlog"]),
        control_db_open=not runtime.closed,
    )


def render_prometheus_text(runtime: MementoRuntime) -> str:
    snapshot = collect_metrics(runtime)
    labels = f'repo_revision="{snapshot.repo_revision}",index_revision="{snapshot.index_revision}"'
    return "\n".join(
        [
            "# HELP memento_service_up Memento process health.",
            "# TYPE memento_service_up gauge",
            f"memento_service_up {snapshot.service_up}",
            "# HELP memento_control_db_open Whether the control database connection is open.",
            "# TYPE memento_control_db_open gauge",
            f"memento_control_db_open {1 if snapshot.control_db_open else 0}",
            "# HELP memento_index_stale Whether the derived index is stale relative to repo head.",
            "# TYPE memento_index_stale gauge",
            f"memento_index_stale {1 if snapshot.index_stale else 0}",
            "# HELP memento_visible_concepts Authorized concept count for the local operator view.",
            "# TYPE memento_visible_concepts gauge",
            f"memento_visible_concepts {snapshot.visible_concepts}",
            "# HELP memento_proposal_backlog Submitted or approved proposals awaiting action.",
            "# TYPE memento_proposal_backlog gauge",
            f"memento_proposal_backlog {snapshot.proposal_backlog}",
            "# HELP memento_repo_revision_info Repo and index revision labels.",
            "# TYPE memento_repo_revision_info gauge",
            f"memento_repo_revision_info{{{''.join(labels)}}} 1",
            "",
        ]
    )
