"""Export synthetic parity fixtures from the pinned Python/uMCP references.

Test tooling only; never imported or executed by the Go runtime.
"""

from __future__ import annotations

import argparse
import importlib
import json
import subprocess
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Any

ROOT = Path(__file__).resolve().parents[2]
BASE = "0b0b8f94dd8b0410a0e3c0fd547e995d2b739b41"
UMCP_PIN = "9c89a708d14ae804e32aa65de10af7c02922617d"
UMCP_TIP = "30cce7dfe08c6ee63de235f7d81754ba286dafbb"
OUT = ROOT / "go/testdata/parity"


def git(path: Path, *args: str) -> str:
    return subprocess.check_output(["git", "-C", str(path), *args], text=True).strip()


def save(name: str, value: Any) -> None:
    (OUT / name).write_text(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True) + "\n")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--umcp", type=Path, required=True)
    args = parser.parse_args()
    if git(args.umcp, "rev-parse", "HEAD") != UMCP_TIP:
        raise SystemExit("uMCP reference HEAD does not match pinned tip")
    for path in ("src", "rust", "pyproject.toml"):
        # Include working-tree changes in the comparison; never bless altered oracles.
        if git(ROOT, "diff", BASE, "--", path):
            raise SystemExit(f"reference {path} differs from pinned baseline")
    if git(args.umcp, "diff", UMCP_TIP, "--", "umcp.py", "aioumcp.py", "umcp_shared.py"):
        raise SystemExit("uMCP source has uncommitted edits")
    sys.path.insert(0, str(ROOT / "src"))
    sys.path.insert(0, str(args.umcp))
    shared = importlib.import_module("umcp_shared")
    extra = importlib.import_module("umcp_contracts")
    envelopes = importlib.import_module("memento.envelopes")
    registry = importlib.import_module("memento.mcp_registry")
    executor = importlib.import_module("memento.executor")
    server = importlib.import_module("memento.server")
    OUT.mkdir(parents=True, exist_ok=True)
    save(
        "baseline.json",
        {
            "memento_commit": BASE,
            "memento_release": "0.5.9",
            "umcp_deployed_commit": UMCP_PIN,
            "umcp_reference_commit": UMCP_TIP,
            "umcp_tip_delta": "documentation only; runtime unchanged from deployed pin",
            "fixture_data": "synthetic/public model fixtures only; no production content",
            "gte_original_go_commit": "d2ffa3a5aaf7be72b178970f48c835c0d8fda5bf",
            "gte_tokenizer_compatibility": "original Go algorithms with Memento Unicode character-boundary fix; no assembly/fast-math",
        },
    )
    cases: list[dict[str, Any]] = [
        {
            "name": "success-default",
            "kind": "success",
            "input": {"data": {"ready": True}, "repo_revision": "r1", "index_revision": "r1"},
        },
        {
            "name": "success-null-data",
            "kind": "success",
            "input": {
                "data": None,
                "repo_revision": "r1",
                "index_revision": "r0",
                "index_stale": True,
                "warnings": ["index delayed"],
                "next_tools": ["memory_status"],
                "operation_id": "op-1",
            },
        },
        {
            "name": "error-default",
            "kind": "error",
            "input": {"error_class": "forbidden", "message": "denied"},
        },
        {
            "name": "error-revisions",
            "kind": "error",
            "input": {
                "error_class": "conflict",
                "message": "changed",
                "repo_revision": "r1",
                "index_revision": "r0",
                "index_stale": True,
                "operation_id": "op-2",
                "warnings": ["reconcile"],
            },
        },
    ]
    for case in cases:
        fn = envelopes.success_envelope if case["kind"] == "success" else envelopes.error_envelope
        case["expected"] = fn(**case["input"]).model_dump(mode="json")
    save("envelopes.json", cases)
    ids: list[Any] = [None, "", "req", 0, -1, 10**40, True, False, 1.0, 1.5, [], {}]
    responses: list[Any] = [
        {},
        [],
        None,
        {"result": 1},
        {"id": None, "result": None},
        {"id": True, "result": 1},
        {"id": 1.0, "result": 1},
        {"id": 10**40, "result": {"ok": True}},
        {"id": "x", "error": {"code": -1, "message": ""}},
        {"id": "x", "error": {"code": True, "message": "bad"}},
        {"id": "x", "error": {"code": 1.0, "message": "bad"}},
        {"id": "x", "error": {"code": 1, "message": None}},
        {"id": "x", "error": None},
        {"id": "x", "error": {}},
        {"id": 1},
        {"id": 1, "result": None, "error": None},
    ]
    save(
        "umcp-shared.json",
        {
            "ids": [{"input": value, "valid": shared.is_valid_jsonrpc_id(value)} for value in ids],
            "responses": [
                {
                    "input": value,
                    "valid": isinstance(value, dict) and shared.is_valid_jsonrpc_response(value),
                }
                for value in responses
            ],
            "versions": [
                {
                    "accepted": value,
                    "preferred": "preferred-custom",
                    "negotiated": shared.exact_or_fallback(value, "preferred-custom"),
                    "error": shared.protocol_version_error(value),
                }
                for value in [None, "", "unknown", "2025-03-26", "2024-11-05"]
            ],
        },
    )
    ops = []
    for spec in registry.OPERATION_SPECS:
        item = asdict(spec)
        item["discovery_surfaces"] = sorted(spec.discovery_surfaces)
        ops.append(item)
    save("mcp-registry.json", ops)
    save("execute-schema.json", executor.execute_plan_schema())
    save(
        "tool-argument-schemas.json",
        {name: model.model_json_schema() for name, model in server._TOOL_ARG_MODELS.items()},
    )
    save(
        "surfaces.json",
        [
            {
                "surface": surface,
                "answer": answer,
                "route": route,
                "tools": registry.tool_names_for_surface(
                    surface, answer_enabled=answer, route_enabled=route
                ),
            }
            for surface in ["compact", "standard", "read_only", "curator", "admin"]
            for answer in [False, True]
            for route in [False, True]
        ],
    )
    save("umcp-http-rules.json", extra.shared_fixtures(shared))
    save("umcp-dispatch.json", extra.dispatch_fixtures(shared))
    save("umcp-progress.json", extra.progress_fixtures(shared))
    pagination = importlib.import_module("umcp_pagination")
    save("umcp-pagination.json", pagination.fixtures(shared))
    stdio = importlib.import_module("umcp_stdio")
    save("umcp-stdio.json", stdio.fixtures(shared))
    urls = importlib.import_module("umcp_urls")
    save("umcp-urls.json", urls.fixtures(shared))
    results = importlib.import_module("umcp_results")
    save("umcp-tool-results.json", results.fixtures())
    tools = importlib.import_module("umcp_tools")
    save("umcp-tools.json", tools.fixtures(shared))
    resources = importlib.import_module("umcp_resources")
    save("umcp-resources.json", resources.fixtures(shared))
    prompts = importlib.import_module("umcp_prompts")
    save("umcp-prompts.json", prompts.fixtures(shared))
    completion = importlib.import_module("umcp_completion")
    save("umcp-completion.json", completion.fixtures(shared))
    http = importlib.import_module("umcp_http")
    save("umcp-http.json", http.fixtures(args.umcp))
    save("umcp-http-modes.json", http.mode_fixtures(args.umcp))
    file_transport = importlib.import_module("umcp_file")
    save("umcp-file.json", file_transport.fixtures(shared))
    tcp_transport = importlib.import_module("umcp_tcp")
    save("umcp-tcp.json", tcp_transport.fixtures(args.umcp))
    sse_transport = importlib.import_module("umcp_sse")
    save("umcp-sse.json", sse_transport.fixtures(args.umcp))
    raw_http = importlib.import_module("umcp_raw_http")
    save("umcp-raw-http.json", raw_http.fixtures(shared))
    notifications = importlib.import_module("umcp_notifications")
    save("umcp-notifications.json", notifications.fixtures())
    sync_http = importlib.import_module("umcp_sync_http")
    save("umcp-sync-http.json", sync_http.fixtures())
    save("umcp-sync-wire.json", sync_http.wire_fixtures(shared))
    cli = importlib.import_module("umcp_cli")
    save("umcp-cli.json", cli.fixtures(shared))
    repository_paths = importlib.import_module("repository_paths")
    save("repository-paths.json", repository_paths.fixtures())
    concept_schema = importlib.import_module("concept_schema")
    save("concept-schema.json", concept_schema.fixtures())
    concept_frontmatter = importlib.import_module("concept_frontmatter")
    save("concept-frontmatter.json", concept_frontmatter.fixtures())
    repository_links = importlib.import_module("repository_links")
    save("repository-links.json", repository_links.fixtures())
    repository_bundle = importlib.import_module("repository_bundle")
    save("repository-bundle.json", repository_bundle.fixtures())
    repository_lease = importlib.import_module("repository_lease")
    save("repository-lease.json", repository_lease.fixtures())
    repository_git = importlib.import_module("repository_git")
    save("repository-git.json", repository_git.fixtures())
    control_db = importlib.import_module("control_db")
    save("control-db.json", control_db.fixtures())
    control_operations = importlib.import_module("control_operations")
    save("control-operations.json", control_operations.fixtures())
    transactions = importlib.import_module("repository_transactions")
    save("repository-transactions.json", transactions.fixtures())
    access_policy = importlib.import_module("access_policy")
    save("access-policy.json", access_policy.fixtures())
    access_store = importlib.import_module("access_store")
    save("access-store.json", access_store.fixtures())
    proposals = importlib.import_module("control_proposals")
    save("control-proposals.json", proposals.fixtures())
    asset_pack = importlib.import_module("asset_pack")
    pack_fixtures = asset_pack.fixtures()
    save("asset-pack.json", pack_fixtures)
    staged_assets = importlib.import_module("staged_assets")
    save("staged-assets.json", staged_assets.fixtures())
    asset_retrieval = importlib.import_module("asset_retrieval")
    save("asset-retrieval.json", asset_retrieval.fixtures())
    accepted_assets = importlib.import_module("accepted_assets")
    save("accepted-assets.json", accepted_assets.fixtures())
    proposal_refresh = importlib.import_module("proposal_refresh")
    save("proposal-refresh.json", proposal_refresh.fixtures())
    proposal_access = importlib.import_module("proposal_access")
    save("proposal-access.json", proposal_access.fixtures())
    proposal_rebase = importlib.import_module("proposal_rebase")
    save("proposal-rebase.json", proposal_rebase.fixtures())
    proposal_summary = importlib.import_module("proposal_summary")
    save("proposal-summary.json", proposal_summary.fixtures())
    proposal_archival = importlib.import_module("proposal_archival")
    save("proposal-archival.json", proposal_archival.fixtures())
    save("proposal-archival-visible.json", proposal_archival.visible_fixtures())
    proposal_review = importlib.import_module("proposal_review")
    save("proposal-review.json", proposal_review.fixtures())
    worktree_mutations = importlib.import_module("worktree_mutations")
    save("worktree-mutations.json", worktree_mutations.fixtures())
    worktree_assets = importlib.import_module("worktree_assets")
    save("worktree-assets.json", worktree_assets.fixtures())
    proposal_preview = importlib.import_module("proposal_preview")
    save("proposal-preview.json", proposal_preview.fixtures())
    proposal_apply = importlib.import_module("proposal_apply")
    save("proposal-apply.json", proposal_apply.fixtures())
    proposal_submit = importlib.import_module("proposal_submit")
    save("proposal-submit.json", proposal_submit.fixtures())
    save("proposal-base64.json", proposal_submit.base64_fixtures())
    proposal_get = importlib.import_module("proposal_get")
    save("proposal-get.json", proposal_get.fixtures())
    proposal_revise = importlib.import_module("proposal_revise")
    save("proposal-revise.json", proposal_revise.fixtures())
    proposal_list = importlib.import_module("proposal_list")
    save("proposal-list.json", proposal_list.fixtures())
    save("proposal-fernet.json", proposal_list.fernet_fixtures())
    save("proposal-fernet-decrypt.json", proposal_list.decrypt_fixtures())
    derived_content = importlib.import_module("derived_content")
    save("derived-content.json", derived_content.fixtures())
    derived_search = importlib.import_module("derived_search")
    save("derived-search.json", derived_search.fixtures())
    derived_graph = importlib.import_module("derived_graph")
    save("derived-graph.json", derived_graph.fixtures())
    derived_lifecycle = importlib.import_module("derived_lifecycle")
    save("derived-lifecycle.json", derived_lifecycle.fixtures())
    service_identity = importlib.import_module("service_identity")
    save("service-identity.json", service_identity.fixtures())
    service_reads = importlib.import_module("service_reads")
    save("service-reads.json", service_reads.fixtures())
    staging_http = importlib.import_module("staging_http")
    save("staging-http.json", staging_http.fixtures())
    staging_tools = importlib.import_module("staging_tools")
    save("staging-tools.json", staging_tools.fixtures())
    service_envelopes = importlib.import_module("service_envelopes")
    save("service-envelopes.json", service_envelopes.fixtures())
    service_workers = importlib.import_module("service_workers")
    save("service-workers.json", service_workers.fixtures())
    operation_get = importlib.import_module("operation_get")
    save("operation-get.json", operation_get.fixtures())
    proposal_tools = importlib.import_module("proposal_tools")
    proposal_tool_fixtures = proposal_tools.fixtures()
    save("proposal-tools.json", proposal_tool_fixtures)
    read_tool_fixtures = proposal_tools.fixtures(
        names=proposal_tools.NAMES + proposal_tools.READ_NAMES
    )
    save("read-tools.json", read_tool_fixtures)
    (ROOT / "go/service/read_tools.json").write_text(
        json.dumps(read_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    staging_tool_fixtures = proposal_tools.fixtures(
        names=proposal_tools.NAMES
        + proposal_tools.READ_NAMES
        + ["memory_asset_stage_begin", "memory_asset_stage_status"]
    )
    save("staging-tool-dispatch.json", staging_tool_fixtures)
    (ROOT / "go/service/staging_tools.json").write_text(
        json.dumps(staging_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    asset_get = importlib.import_module("asset_get")
    save("asset-get.json", asset_get.fixtures())
    asset_prune = importlib.import_module("asset_prune")
    save("asset-prune.json", asset_prune.fixtures())
    asset_tool_fixtures = proposal_tools.fixtures(
        names=proposal_tools.NAMES
        + proposal_tools.READ_NAMES
        + [
            "memory_asset_stage_begin",
            "memory_asset_stage_status",
            "memory_asset_get",
            "memory_asset_prune",
        ]
    )
    save("asset-tool-dispatch.json", asset_tool_fixtures)
    (ROOT / "go/service/asset_tools.json").write_text(
        json.dumps(asset_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    direct_mutations = importlib.import_module("direct_mutations")
    save("direct-mutations.json", direct_mutations.fixtures())
    mutation_tool_fixtures = proposal_tools.fixtures(
        names=proposal_tools.NAMES
        + proposal_tools.READ_NAMES
        + [
            "memory_asset_stage_begin",
            "memory_asset_stage_status",
            "memory_asset_get",
            "memory_asset_prune",
            "memory_create",
            "memory_patch",
            "memory_rename",
        ]
    )
    save("mutation-tool-dispatch.json", mutation_tool_fixtures)
    (ROOT / "go/service/mutation_tools.json").write_text(
        json.dumps(mutation_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    trash_mutations = importlib.import_module("trash_mutations")
    save("trash-mutations.json", trash_mutations.fixtures())
    trash_tool_fixtures = proposal_tools.fixtures(
        names=proposal_tools.NAMES
        + proposal_tools.READ_NAMES
        + [
            "memory_asset_stage_begin",
            "memory_asset_stage_status",
            "memory_asset_get",
            "memory_asset_prune",
            "memory_create",
            "memory_patch",
            "memory_rename",
            "memory_trash",
            "memory_restore",
            "memory_purge",
        ]
    )
    save("trash-tool-dispatch.json", trash_tool_fixtures)
    (ROOT / "go/service/trash_tools.json").write_text(
        json.dumps(trash_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    # Runtime embeds only definitions for implemented tools, not test outcomes.
    (ROOT / "go/service/proposal_tools.json").write_text(
        json.dumps(proposal_tool_fixtures["definitions"], indent=2, sort_keys=True) + "\n"
    )
    (ROOT / "go/assets/mime.json").write_text(
        json.dumps(pack_fixtures["mime"], ensure_ascii=False, sort_keys=True, indent=2) + "\n"
    )
    print(f"Exported pinned synthetic fixtures to {OUT}")


if __name__ == "__main__":
    main()
