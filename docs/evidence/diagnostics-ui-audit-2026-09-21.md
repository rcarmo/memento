# Graph diagnostics and selection audit — 21 September 2026

The production graph's 69 reported broken edges included 53 valid asset links. The API actually classified 16 explicit links as broken. The combined correction fixes that total, selection-scoped reporting, incomplete-data handling and navigation; it does not rewrite stored Markdown.

## Production reproduction

The user reported repeated diagnostics, a selected linked node appearing orphaned, failed link retrieval, non-navigating findings/neighbours, ascending asset versions and centred sidebar content. The named example was **Website Device Screenshots**, full diagnostic view, ID `687bd0cf-42c0-4c8c-bed5-f973b6037d99`.

Read-only API and Chromium checks against production v1.0.5 found:

- detail HTTP 200, `orphan=false`, explicit inbound/outbound degrees 0/1;
- one resolved internal link to `/skills/playwright.md`, plus three external URLs;
- no broken link for that node;
- a size-outlier diagnostic for that node;
- the sidebar still displayed the first 50 global diagnostics after selection, including unrelated orphan findings.

The original live detail failure did not recur. The corrected tests inject detail and neighbourhood failures independently. A live request took about five seconds, so success in this sample does not establish that the earlier failure was imaginary or that latency is fixed.

The live explicit edge classifications were 396 resolved, 51 external, 53 asset and 16 broken. The remaining broken destinations are twelve `../visual-information-design/SKILL.md` references, two `../technical-docs/SKILL.md` references, one `../graph-design/SKILL.md`, and one workspace-relative `../../../piclaw/runtime/skills/builtin/remote-peer/SKILL.md`. Some names resemble current concepts, but their relative paths do not resolve in the stored repository. No inferred target substitutions were made.

## Confirmed defects and corrections

| Defect | Correction and regression |
| --- | --- |
| Overview counted accepted asset edges as broken while node diagnostics excluded them | The overview total uses the same explicit/external/asset classification as node reporting. `TestOverviewAcceptedAssetDoesNotCountBroken`. |
| Sidebar showed global repeated messages during node selection | Scope diagnostics to the selected node/cluster or filtered view, deduplicate by diagnostic ID, and display explicit target buttons. Unit tests cover filtered aggregate memberships and multi-target clipping. |
| Detail failure became zero inbound/outbound links, no assets and no proposals | Display unavailable/loading separately from empty data, retain error feedback and provide retry. Browser failure injection verifies no fabricated zero/empty state. |
| Truncated edge lists could make a linked node appear orphaned | Calculate degree/orphan/broken findings against the complete permitted explicit edge set before capping display lists. Add truncation metadata and UI warnings. Detail, overview, cluster and neighbourhood cap regressions. |
| Drill-downs ignored the Show Trash setting | Propagate the setting into cluster, detail and neighbourhood options; test included/excluded trash peers. |
| Detail diagnostics bypassed the configured node scope | Apply the configured refresh node budget, retain the selected centre and report truncation. |
| Background polling erased neighbourhood-failure feedback | Retry neighbourhood navigation after embedding progress; preserve successful detail and retry feedback while the neighbourhood still fails. |
| Neighbourhood configuration omitted its node/semantic-edge budgets | Propagate configured budgets; retain the requested centre when limiting neighbours. |
| Drill-down reused diagnostics from an older graph | Detail, cluster and neighbourhood responses include fresh scoped diagnostics, with target IDs restricted to the returned selection. The UI consumes that response instead of retaining the previous snapshot's findings. |
| Successful detail was discarded when neighbourhood loading failed | Retain the loaded inspector, show a separate neighbourhood error, reveal the selected node alone with a truncation warning and retry neighbourhood loading explicitly. |
| Navigation completion erased search text entered while loading | Clear filters when navigation starts, not when the asynchronous request completes. |
| Findings were plain text and selection/navigation state diverged | Diagnostic target buttons and semantic-neighbour buttons use the same memory navigation path. Multi-node findings show their individual permitted targets. Browser tests cover click and keyboard activation. |
| Asset summaries sorted filenames ascending and applied limit before sorting | Sort by asset kind and descending stable semantic version, then limit. Invalid versions have deterministic fallback ordering. `1.0.10` precedes `1.0.2`; limit selects the newest version. |
| Transient graph read failures were returned as unknown-node 404s | Known snapshot errors retain 404; unexpected storage/read errors return redacted retryable 503. |
| Sidebar content inherited centred button text | Explicit left alignment for controls/inspector text, buttons, inputs, summaries and action groups; browser computed-style assertions. |

## Scope and semantics retained

An orphan has no resolved concept-to-concept links within the permitted snapshot. External URLs, accepted asset files and semantic-similarity overlays do not establish explicit connectivity. The UI explains this distinction. Existing directed self-link degree semantics remain unchanged.

Full-snapshot diagnostic messages and measurements retain their meaning when target buttons are clipped to a selection. A `scope_limited` flag labels that case. A diagnostic is not duplicated merely because separate nodes share its message; IDs determine duplicates.

Expanded views remain subsets until the user chooses Overview. The controls now distinguish “Show current view diagnostics” from “Return to overview diagnostics”. API display caps are reported explicitly; configuration still bounds node scope. Diagnostic calculations read all explicit links for that permitted node scope and response edge lists remain capped.

The link resolver's separate handling of percent-encoded filenames and query strings deserves its own behavioural decision: it currently resolves literal stored targets after fragment removal. None of the sixteen live unresolved links uses that syntax. This patch does not change Markdown rewrite/parity semantics as an incidental UI fix.

## Verification

- Full local `make quality`: 100% production statement coverage, vet and Staticcheck pass.
- `make race vuln cross`: passes; no called-code vulnerabilities reported.
- Python/Node-disabled `make python-parity`: passes. Additive diagnostics/truncation fields are tested explicitly while retained fixture fields remain compared.
- Playwright: 18/18 scenarios pass in Chromium and 18/18 in WebKit, plus three diagnostics scope/summary unit tests.
- New browser scenario covers selected findings, deduplication, diagnostic and semantic navigation, keyboard activation, failed detail retry, off-graph neighbourhood failure preserving detail, version order and sidebar alignment.
- Existing browser cases cover stale selection/principal/search responses, overview races, pointer/touch, exports, admin and empty/WebGL-unavailable views.
- Independent review identified scope and navigation gaps; the confirmed issues above were fixed and reproduced. A broad review timed out and was not counted as a pass. A subsequent focused review identified the detail budget, trash drill-down and polling issues; all three have regressions.
- Reproducibility: locked Playwright 1.61.0, shared `make ui-setup` / `make ui-test UI_REPEAT=2` local and CI commands, isolated run directories, controlled polling clock, request barriers, failure traces and interruption cleanup regression. Two runs per engine passed in a clean source copy under `/tmp` with no inherited project dependencies or state (72 browser scenario executions); dependency/toolchain caches were reused.
- UX inventory: 24 domain-level Gherkin scenarios with checked evidence references; 19 automated, three partial and two documented-only. Gherkin steps are not independently executed. `make ux-check` rejects binding drift.

Evidence captured under `/workspace/tmp/memento-link-audit/`; repeatable tests are checked into `internal/graphdebug/` and `tools/browser/`. Production remains v1.0.5 until exact-commit CI, release and target validation complete.
