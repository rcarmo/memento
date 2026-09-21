Feature: ux/graph visual debugger

  These scenarios describe the current browser behaviour of the maintained graph debugger.
  Scenario IDs stay stable. Automation and evidence mappings live in testdata/features/ux/scenario-inventory.json.

  Rule: controls and navigation

    @ux-graph-001
    Scenario: Graph debugger opens with trusted-network warning and core controls
      Given a Memento instance exposes the graph debugger on a trusted network
      When an operator opens the debugger
      Then the page shows the current graph canvas and warns that the surface is unauthenticated
      And the operator can filter or tune the view by search text, simulated principal, memory type, size metric, semantic settings and layout forces

    @ux-graph-002
    Scenario: Search, tag selection and overview reset keep navigation local to the current graph
      Given the debugger is showing a graph snapshot
      When an operator searches, follows a result, selects a tag, or returns to the overview
      Then the inspector follows the selected memory
      And tag-driven filtering and overview reset affect only the current graph view

    @ux-graph-003
    Scenario: Simulated principals and trash toggles rescope the visible graph and suspend refresh
      Given the debugger can show the full graph or a simulated principal view
      When an operator switches principal simulation or includes trash
      Then the visible nodes and links are rebuilt for that scope
      And embedding refresh actions stay unavailable while visibility is being simulated

    @ux-graph-004
    Scenario: Semantic overlays and force controls adjust the current view without discarding filters
      Given the debugger is showing a filtered graph view
      When an operator enables semantic overlays or tunes semantic and force controls
      Then the rendered relationships and layout update in place
      And later refresh completion preserves the current semantic and filter settings

    @ux-graph-005
    Scenario: Aggregate clusters can expand into direct members and suppress aggregate-only actions
      Given the debugger is showing an aggregate overview
      When an operator opens a cluster
      Then the view can drill into direct members and their bounded relationships
      And actions that require direct visible memories remain unavailable until the cluster is expanded

  Rule: diagnostics, loading and export

    @ux-graph-006
    Scenario: Diagnostics stay scoped, deduplicated and navigable from the current view or selection
      Given diagnostics exist for the current graph snapshot
      When an operator reviews the current view, a selected node, or a selected cluster
      Then the debugger lists only findings that apply within that scope
      And duplicate findings collapse to one entry with target links that open the relevant memories

    @ux-graph-007
    Scenario: Detail and neighbourhood failures stay visible, distinct from empty state, and retryable
      Given the debugger is loading a selected memory
      When detail or neighbourhood retrieval fails
      Then the inspector keeps a visible error instead of pretending that relationships or assets are empty
      And the operator can retry the selected memory without losing the current context

    @ux-graph-008
    Scenario: Slow overview, search and node replies cannot repopulate a newer view
      Given a graph request is still in flight
      When an operator changes principal simulation, returns to the overview, or starts a newer navigation path
      Then late replies from the older request do not overwrite the newer graph, inspector or search state

    @ux-graph-009
    Scenario: Embedding refresh supports selected, visible and full scopes with confirmation and worker feedback
      Given embedding refresh is available for the live graph view
      When an operator refreshes the selected memory, the visible view, or the full derived graph
      Then the request scope matches the chosen action
      And full refresh requires confirmation while worker failures stay visible in the page state

    @ux-graph-010
    Scenario: Exports follow the current filtered view and disable bounded formats when nothing is visible
      Given the debugger is showing the current filtered graph view
      When an operator exports the view
      Then PNG uses the current camera view while SVG and JSON stay bounded to explicit graph data and metadata
      And bounded file exports stay unavailable when no visible memories can be exported

    @ux-graph-011
    Scenario: Pointer, wheel and touch gestures support hover, pick and zoom without pinch misselection
      Given the debugger is rendering a graph in the canvas
      When an operator hovers, clicks, scrolls or performs a multi-touch gesture
      Then hover labels, selection and zoom follow the single-pointer interactions
      And pinch or cancelled touch gestures do not create accidental selections

    @ux-graph-012
    Scenario: Empty, failed and WebGL-unavailable graph states surface clear fallback messaging
      Given the graph view may be empty or degraded
      When there are no visible memories, the API fails, or WebGL2 is unavailable
      Then the page keeps a usable empty or fallback state
      And failures remain visible until the operator dismisses or retries them

    @ux-graph-013
    Scenario: The sidebar keeps legend and live telemetry visible during graph inspection
      Given the debugger is rendering a graph
      When an operator reviews the sidebar
      Then the page shows the legend for node and link styles
      And the page shows live node, edge, culling, LOD, frame-rate and fetch counters

    @ux-graph-014
    Scenario: Named controls and keyboard activation remain available for non-canvas navigation
      Given an operator is using the debugger without relying on canvas gestures alone
      When the operator uses labelled controls, dialog actions and keyboard focus
      Then named inputs and buttons remain available for search, view changes, refresh and export
      And non-canvas navigation targets can be activated from keyboard focus

    @ux-graph-015
    Scenario: Re-entering the debugger does not retain stale overlay UI from a previous session
      Given a graph session has ended or the page has been torn down
      When the debugger is entered again
      Then transient hover and cluster overlays from the previous session are gone
      And the new session starts with only the current graph state
