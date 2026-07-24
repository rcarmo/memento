import { h, render } from "./vendor/preact.module.js";
import { useEffect, useMemo, useRef, useState } from "./vendor/preact-hooks.module.js";
import { graphApi } from "./api.js";
import { GraphScene } from "./graph-scene.js";

const forceDefaults = {
  explicit: 0.06,
  semantic_similarity: 0.027,
  shared_tag: 0.009,
  shared_namespace: 0.018,
  shared_type: 0.006,
  shared_provenance: 0.006,
};
const simulatedWarning = "Simulated visibility — not an authorization boundary";
const simulatedRefreshWarning = "Embedding refresh is disabled while simulating visibility.";

function normalizePrincipals(payload) {
  const items = Array.isArray(payload)
    ? payload
    : Array.isArray(payload?.principals)
      ? payload.principals
      : [];
  return items
    .filter((item) => item && typeof item === "object")
    .map((item) => ({
      name: typeof item.name === "string" ? item.name : "",
      roles: Array.isArray(item.roles) ? item.roles.filter((value) => typeof value === "string") : [],
      read_prefixes: Array.isArray(item.read_prefixes)
        ? item.read_prefixes.filter((value) => typeof value === "string")
        : Array.isArray(item.readPrefixes)
          ? item.readPrefixes.filter((value) => typeof value === "string")
          : [],
      write_prefixes: Array.isArray(item.write_prefixes)
        ? item.write_prefixes.filter((value) => typeof value === "string")
        : Array.isArray(item.writePrefixes)
          ? item.writePrefixes.filter((value) => typeof value === "string")
          : [],
    }))
    .filter((item) => item.name);
}

function principalRoles(principal) {
  return principal.roles.length ? principal.roles.join(", ") : "none";
}

function principalPrefixes(prefixes) {
  return prefixes.length ? prefixes.join(", ") : "none";
}

function principalOptionLabel(principal) {
  return `${principal.name} — roles: ${principalRoles(principal)}`;
}

function principalDetail(principal) {
  return `Roles: ${principalRoles(principal)} · Read: ${principalPrefixes(principal.read_prefixes)} · Write: ${principalPrefixes(principal.write_prefixes)}`;
}

function App() {
  const canvas = useRef(null);
  const scene = useRef(null);
  const exportDialog = useRef(null);
  const [graph, setGraph] = useState(null);
  const [selected, setSelected] = useState(null);
  const [detail, setDetail] = useState(null);
  const [error, setError] = useState(null);
  const [query, setQuery] = useState("");
  const [searchResults, setSearchResults] = useState([]);
  const [searching, setSearching] = useState(false);
  const [type, setType] = useState("all");
  const [sizeMetric, setSizeMetric] = useState("combined_bytes");
  const [forces, setForces] = useState(forceDefaults);
  const [perf, setPerf] = useState({});
  const [timing, setTiming] = useState({});
  const [refresh, setRefresh] = useState(null);
  const [includePreview, setIncludePreview] = useState(false);
  const [principals, setPrincipals] = useState([]);
  const [simulatedPrincipal, setSimulatedPrincipal] = useState("");

  useEffect(() => {
    try {
      if (!canvas.current.getContext("webgl2")) {
        throw new Error("WebGL2 is unavailable in this browser or graphics environment.");
      }
      scene.current = new GraphScene(canvas.current, { select: (node) => selectNode(node), performance: setPerf });
      window.__mementoGraphScene = scene.current;
      loadPrincipals();
      load();
      loadRefreshStatus();
    } catch (e) {
      setError(`Visual debugger unavailable: ${e.message}`);
    }
    return () => {
      delete window.__mementoGraphScene;
      scene.current?.worker?.terminate();
    };
  }, []);

  async function loadPrincipals() {
    try {
      const { payload } = await graphApi.principals();
      setPrincipals(normalizePrincipals(payload));
    } catch (e) {
      setPrincipals([]);
      setError(e.message);
    }
  }

  async function load() {
    try {
      const { payload, elapsed, bytes } = await graphApi.overview();
      setTiming({ fetch: elapsed, bytes });
      setGraph(payload);
      draw(payload);
    } catch (e) {
      setError(e.message);
    }
  }

  async function loadRefreshStatus() {
    try {
      const { payload } = await graphApi.refreshStatus();
      setRefresh(payload);
    } catch (e) {
      setRefresh({ available: false, last_error: e.message });
    }
  }

  function draw(payload) {
    const aggregated = payload.mode === "aggregated";
    const nodes = aggregated ? payload.clusters : payload.nodes;
    const edges = aggregated
      ? payload.cluster_edges.map((edge) => ({ ...edge, kind: edge.kind || "explicit" }))
      : payload.edges;
    scene.current?.setGraph(nodes, edges, { sizeMetric, forces });
  }

  async function selectNode(node) {
    setSelected(node);
    scene.current?.focus(node);
    try {
      if (node.member_count) {
        const { payload } = await graphApi.cluster(node.id);
        setGraph((current) => ({ ...current, mode: "direct", nodes: payload.nodes, edges: payload.edges }));
        draw({ mode: "direct", nodes: payload.nodes, edges: payload.edges });
        setDetail({ cluster: true, ...payload });
      } else {
        const { payload } = await graphApi.detail(node.id);
        setDetail(payload);
      }
    } catch (e) {
      setError(e.message);
    }
  }

  async function openMemory(id) {
    if (!id) return;
    try {
      const { payload } = await graphApi.detail(id);
      const present = graph?.mode === "direct" && graph.nodes?.some((node) => node.id === id);
      if (!present) {
        const { payload: hood } = await graphApi.neighbourhood(id);
        const revealed = {
          ...graph,
          mode: "direct",
          nodes: hood.nodes,
          edges: hood.edges,
          clusters: [],
          cluster_edges: [],
        };
        setGraph(revealed);
        draw(revealed);
      }
      setQuery("");
      setSearchResults([]);
      setType("all");
      setSelected(payload.node);
      setDetail(payload);
      scene.current?.focus(payload.node);
    } catch (e) {
      setError(e.message);
    }
  }

  useEffect(() => {
    if (graph) draw(graph);
  }, [sizeMetric, forces]);

  useEffect(() => {
    const value = query.trim();
    if (value.length < 2) {
      setSearchResults([]);
      setSearching(false);
      return;
    }
    let cancelled = false;
    setSearching(true);
    const timer = setTimeout(async () => {
      try {
        const { payload } = await graphApi.search(value);
        if (!cancelled) setSearchResults(payload.results || []);
      } catch (e) {
        if (!cancelled) setError(e.message);
      } finally {
        if (!cancelled) setSearching(false);
      }
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [query]);

  const nodes = graph ? (graph.mode === "aggregated" ? graph.clusters : graph.nodes) : [];
  const aggregateType = (node) =>
    node.namespace === "/skills/" ? "skill" : (node.type_counts?.[0]?.[0] || node.type);
  const matchesType = (node) => type === "all" || node.type === type || aggregateType(node) === type;
  const matchesQuery = (node) =>
    !query || `${node.title || ""} ${node.label || ""} ${node.path || ""} ${node.namespace || ""} ${(node.tags || []).join(" ")}`.toLowerCase().includes(query.toLowerCase());
  const filtered = useMemo(() => nodes.filter((node) => matchesType(node) && matchesQuery(node)), [nodes, type, query]);

  function redrawFiltered() {
    if (!graph) return;
    const visible = new Set(filtered.map((node) => node.id));
    const sourceEdges = graph.mode === "aggregated" ? graph.cluster_edges : graph.edges;
    const filteredEdges = sourceEdges
      .filter((edge) => visible.has(edge.source) && visible.has(edge.target))
      .map((edge) => (graph.mode === "aggregated" ? { ...edge, kind: edge.kind || "explicit" } : edge));
    scene.current?.setGraph(filtered, filteredEdges, { sizeMetric, forces });
  }

  useEffect(redrawFiltered, [filtered]);

  async function changeView(name) {
    const next = typeof name === "string" ? name.trim() : "";
    setSelected(null);
    setDetail(null);
    setQuery("");
    setSearchResults([]);
    setSearching(false);
    scene.current && (scene.current.selectedId = null);
    scene.current?.drawHalos?.();
    setSimulatedPrincipal(next);
    graphApi.setSimulatedPrincipal(next);
    await Promise.all([load(), loadRefreshStatus()]);
  }

  async function refreshEmbedding(scope) {
    if (simulatedPrincipal) {
      setError(simulatedRefreshWarning);
      return;
    }
    try {
      if (refresh && refresh.available === false) {
        throw new Error(refresh.last_error || "Semantic embedding refresh is unavailable on this Memento instance.");
      }
      const ids = scope === "selected" && selected
        ? [selected.id]
        : filtered.filter((node) => !node.member_count).map((node) => node.id);
      const { payload } = await graphApi.refresh(scope, ids, scope === "full");
      setRefresh(payload);
    } catch (e) {
      setError(e.message);
      await loadRefreshStatus();
    }
  }

  function download(name, href) {
    const anchor = document.createElement("a");
    anchor.download = name;
    anchor.href = href;
    anchor.click();
  }

  const exportIds = () => filtered.filter((node) => !node.member_count).map((node) => node.id);

  async function exportFormat(format) {
    try {
      const settings = { query, type, sizeMetric, forces, include_preview: includePreview };
      if (format === "png") {
        download("memento-graph.png", scene.current.exportPng());
      } else {
        const blob = format === "svg"
          ? await graphApi.exportSvg(exportIds(), settings)
          : await graphApi.exportJson(exportIds(), settings);
        download(`memento-graph.${format}`, URL.createObjectURL(blob));
      }
      exportDialog.current?.close();
    } catch (e) {
      setError(e.message);
    }
  }

  const icon = (kind) =>
    kind === "camera"
      ? h("svg", { viewBox: "0 0 24 24", "aria-hidden": "true" }, [
          h("path", { d: "M4 7h3l2-3h6l2 3h3v13H4z" }),
          h("circle", { cx: 12, cy: 13, r: 4 }),
        ])
      : kind === "image"
        ? h("svg", { viewBox: "0 0 24 24", "aria-hidden": "true" }, [
            h("rect", { x: 3, y: 4, width: 18, height: 16, rx: 2 }),
            h("circle", { cx: 8, cy: 9, r: 2 }),
            h("path", { d: "m5 18 5-5 3 3 2-2 4 4" }),
          ])
        : kind === "vector"
          ? h("svg", { viewBox: "0 0 24 24", "aria-hidden": "true" }, [
              h("circle", { cx: 5, cy: 12, r: 2 }),
              h("circle", { cx: 19, cy: 6, r: 2 }),
              h("circle", { cx: 19, cy: 18, r: 2 }),
              h("path", { d: "m7 12 10-6M7 12l10 6" }),
            ])
          : h("svg", { viewBox: "0 0 24 24", "aria-hidden": "true" }, [
              h("path", { d: "M5 3h10l4 4v14H5zM15 3v5h5" }),
              h("path", { d: "M8 12h8M8 16h8" }),
            ]);

  const activePrincipal = principals.find((principal) => principal.name === simulatedPrincipal) || null;
  const refreshUnavailable = refresh?.available === false
    ? refresh.last_error || "Semantic embedding refresh is unavailable on this Memento instance."
    : "";
  const refreshDisabledTitle = simulatedPrincipal
    ? simulatedRefreshWarning
    : refresh?.available === false
      ? "Semantic embedding refresh is unavailable on this instance"
      : "";

  return h("div", { class: "app" }, [
    h("header", {}, [
      h("strong", {}, "Memento Visual Debugger"),
      h(
        "span",
        { class: `warning${simulatedPrincipal ? " simulated" : ""}` },
        simulatedPrincipal ? simulatedWarning : "Unauthenticated -- trusted networks only",
      ),
      h("button", { onClick: load }, "Overview"),
    ]),
    h("aside", { class: "controls" }, [
      h("label", { class: "search-control" }, [
        "Search",
        h("input", {
          value: query,
          onInput: (event) => setQuery(event.currentTarget.value),
          placeholder: "title, tags, full text",
        }),
        searching && h("small", {}, "Searching…"),
        searchResults.length
          ? h(
              "ul",
              { class: "search-results" },
              searchResults.map((result) =>
                h("li", {},
                  h("button", { onClick: () => openMemory(result.id) }, [
                    h("strong", {}, result.title),
                    h("code", {}, result.path),
                    ...(result.tags || []).map((tag) => h("span", { class: "tag" }, tag)),
                    result.snippet && h("small", {}, result.snippet),
                  ]),
                ),
              ),
            )
          : null,
      ]),
      h("label", {}, [
        "View as",
        h(
          "select",
          { value: simulatedPrincipal, onChange: (event) => changeView(event.currentTarget.value) },
          [
            h("option", { value: "" }, "Full diagnostic"),
            ...principals.map((principal) =>
              h("option", { value: principal.name }, principalOptionLabel(principal)),
            ),
          ],
        ),
        h(
          "small",
          { class: "selection-detail" },
          activePrincipal
            ? principalDetail(activePrincipal)
            : "All nodes and edges visible for diagnostics.",
        ),
      ]),
      h("label", {}, [
        "Type",
        h(
          "select",
          { value: type, onChange: (event) => setType(event.currentTarget.value) },
          ["all", "project", "instance", "person", "service", "system", "skill"].map((value) =>
            h("option", { value }, value),
          ),
        ),
      ]),
      h("label", {}, [
        "Size",
        h(
          "select",
          { value: sizeMetric, onChange: (event) => setSizeMetric(event.currentTarget.value) },
          ["combined_bytes", "markdown_bytes", "asset_bytes", "explicit_in_degree", "explicit_out_degree", "member_count"].map((value) =>
            h("option", { value }, value),
          ),
        ),
      ]),
      h("details", {}, [
        h("summary", {}, "Forces"),
        ...Object.entries(forces).map(([key, value]) =>
          h("label", { class: "slider" }, [
            key,
            h("input", {
              type: "range",
              min: 0,
              max: 0.25,
              step: 0.001,
              value,
              onInput: (event) => setForces({ ...forces, [key]: Number(event.currentTarget.value) }),
            }),
          ]),
        ),
      ]),
      h("details", { open: true }, [
        h("summary", {}, `Diagnostics (${graph?.diagnostics?.length || 0})`),
        h(
          "ul",
          { class: "diagnostics" },
          (graph?.diagnostics || []).slice(0, 50).map((diagnostic) =>
            h(
              "li",
              { class: diagnostic.severity, title: JSON.stringify(diagnostic.measured) },
              `${diagnostic.rule}: ${diagnostic.message}`,
            ),
          ),
        ),
      ]),
      h("div", { class: "actions" }, [
        h(
          "button",
          {
            disabled: Boolean(simulatedPrincipal || refresh?.available === false || !selected || selected.member_count),
            onClick: () => refreshEmbedding("selected"),
            title: refreshDisabledTitle,
          },
          "Refresh selected embedding",
        ),
        h(
          "button",
          {
            disabled: Boolean(simulatedPrincipal || refresh?.available === false),
            onClick: () => refreshEmbedding("visible"),
            title: refreshDisabledTitle,
          },
          "Refresh visible",
        ),
        h(
          "button",
          {
            disabled: Boolean(simulatedPrincipal || refresh?.available === false),
            onClick: () => confirm("Refresh all derived embeddings?") && refreshEmbedding("full"),
            title: refreshDisabledTitle,
          },
          "Refresh all",
        ),
        simulatedPrincipal && h("p", { class: "warning" }, simulatedRefreshWarning),
        !simulatedPrincipal && refreshUnavailable && h("p", { class: "warning" }, refreshUnavailable),
      ]),
      h(
        "button",
        {
          class: "icon-button",
          onClick: () => exportDialog.current?.showModal(),
          title: "Export current graph",
          "aria-label": "Export current graph",
        },
        [icon("camera"), h("span", {}, "Export")],
      ),
      h(
        "dialog",
        {
          ref: exportDialog,
          class: "export-dialog",
          onClick: (event) => {
            if (event.target === event.currentTarget) event.currentTarget.close();
          },
        },
        h("form", { method: "dialog" }, [
          h("header", {}, [
            h("h2", {}, "Export graph"),
            h("button", { value: "cancel", "aria-label": "Close export dialog" }, "×"),
          ]),
          h("p", {}, "Choose a format for the current filtered view."),
          h("div", { class: "format-grid" }, [
            h("button", { type: "button", onClick: () => exportFormat("png") }, [
              icon("image"),
              h("strong", {}, "PNG"),
              h("small", {}, "Current camera and colours"),
            ]),
            h("button", { type: "button", disabled: !exportIds().length, onClick: () => exportFormat("svg") }, [
              icon("vector"),
              h("strong", {}, "SVG"),
              h("small", {}, "Bounded memory neighbourhood"),
            ]),
            h("button", { type: "button", disabled: !exportIds().length, onClick: () => exportFormat("json") }, [
              icon("data"),
              h("strong", {}, "JSON"),
              h("small", {}, "Nodes, edges, settings and revisions"),
            ]),
          ]),
          h("label", { class: "check" }, [
            h("input", {
              type: "checkbox",
              checked: includePreview,
              onChange: (event) => setIncludePreview(event.currentTarget.checked),
            }),
            "Include bounded preview where allowed",
          ]),
        ]),
      ),
      h("dl", { class: "perf" }, [
        h("dt", {}, "Nodes"),
        h("dd", {}, perf.nodes || 0),
        h("dt", {}, "Edges"),
        h("dd", {}, perf.edges || 0),
        h("dt", {}, "Culled"),
        h("dd", {}, perf.culledEdges || 0),
        h("dt", {}, "LOD"),
        h("dd", {}, perf.lod || "near"),
        h("dt", {}, "FPS"),
        h("dd", {}, (perf.fps || 0).toFixed(1)),
        h("dt", {}, "Fetch"),
        h("dd", {}, `${(timing.fetch || 0).toFixed(0)} ms`),
        h("dt", {}, "Bytes"),
        h("dd", {}, timing.bytes || 0),
      ]),
    ]),
    h("main", { class: "viewport" }, [
      h("canvas", { ref: canvas, tabIndex: 0, "aria-label": "2.5D memory graph" }),
      h("div", { class: "legend", "aria-label": "Graph legend" }, [
        h("span", {}, [h("i", { class: "shape sphere" }), "memory"]),
        h("span", {}, [h("i", { class: "shape box" }), "system / service"]),
        h("span", {}, [h("i", { class: "shape diamond" }), "high degree"]),
        h("span", {}, [h("i", { class: "shape ring" }), "cluster"]),
        h("span", {}, [h("i", { class: "line explicit" }), "explicit link"]),
        h("span", {}, [h("i", { class: "line semantic" }), "semantic overlay"]),
      ]),
      error && h("div", { class: "toast-error", onClick: () => setError(null) }, error),
    ]),
    h(
      "section",
      { class: "inspector" },
      detail
        ? h(Inspector, {
            detail,
            selected,
            onTag: (tag) => {
              setType("all");
              setQuery(tag);
            },
            onMemory: openMemory,
          })
        : h("p", {}, "Select a memory or cluster to inspect provenance and relationships."),
    ),
  ]);
}

function Inspector({ detail, selected, onTag, onMemory }) {
  const node = detail.node || selected;
  const tags = node.tags || [];
  const edgeLine = (edge, id) =>
    h("li", {}, [
      h("button", { class: "link-button", disabled: !id, onClick: () => id && onMemory(id) }, edge.raw_target || id || "missing"),
      " ",
      h("small", {}, edge.resolution || edge.kind || ""),
    ]);
  const proposalLine = (proposal) =>
    h("li", {}, [
      h("code", {}, proposal.status),
      " ",
      proposal.intent || proposal.proposal_id,
      " ",
      h("small", {}, proposal.author || ""),
    ]);
  const assetLine = (asset) =>
    h("li", {}, [h("code", {}, `${asset.asset_kind}:${asset.version}`), ` ${asset.payload_bytes || 0} bytes`]);
  const tagButton = (tag) => h("button", { class: "tag", onClick: () => onTag(tag) }, tag);
  const members = detail.nodes
    ? h("details", { open: true }, [
        h("summary", {}, `Cluster members (${detail.nodes.length})`),
        h(
          "ul",
          {},
          detail.nodes.slice(0, 80).map((member) =>
            h("li", {}, [
              h("button", { class: "link-button", onClick: () => onMemory(member.id) }, member.path),
              " ",
              member.title,
              " ",
              (member.tags || []).map(tagButton),
            ]),
          ),
        ),
      ])
    : null;

  return h("div", {}, [
    h("h2", {}, node.title || node.label),
    h("code", {}, node.path || node.namespace),
    tags.length ? h("p", { class: "tags" }, tags.map(tagButton)) : null,
    h(
      "dl",
      {},
      Object.entries({
        type: node.type,
        status: node.status,
        namespace: node.namespace,
        members: node.member_count,
        updated: node.updated_at,
        updated_by: node.updated_by,
        markdown_bytes: node.markdown_bytes,
        asset_bytes: node.asset_bytes,
        proposals: node.proposal_count,
        embedding: node.embedding?.status,
      })
        .filter(([, value]) => value != null)
        .flatMap(([key, value]) => [h("dt", {}, key), h("dd", {}, String(value))]),
    ),
    detail.preview && h("pre", { class: "preview" }, detail.preview),
    members,
    h("h3", {}, "Explicit links"),
    h("p", {}, `${detail.inbound?.length || 0} inbound / ${detail.outbound?.length || 0} outbound`),
    h("div", { class: "link-lists" }, [
      detail.inbound?.length
        ? h("div", {}, [
            h("h4", {}, "Inbound"),
            h("ul", {}, detail.inbound.slice(0, 30).map((edge) => edgeLine(edge, edge.source))),
          ])
        : null,
      detail.outbound?.length
        ? h("div", {}, [
            h("h4", {}, "Outbound"),
            h("ul", {}, detail.outbound.slice(0, 30).map((edge) => edgeLine(edge, edge.target))),
          ])
        : null,
    ]),
    h("h3", {}, "Assets / proposals"),
    detail.assets?.length ? h("ul", {}, detail.assets.map(assetLine)) : h("p", {}, "No assets."),
    detail.proposals?.length ? h("ul", {}, detail.proposals.map(proposalLine)) : h("p", {}, "No proposals."),
  ]);
}

render(h(App), document.getElementById("app"));
