from __future__ import annotations

import hashlib
import json
from importlib.resources import files


def test_graph_application_assets_are_available_as_package_resources() -> None:
    root = files("memento.graph_debug").joinpath("static")
    for relative in (
        "index.html",
        "app.css",
        "app.js",
        "api.js",
        "graph-scene.js",
        "layout-worker.js",
    ):
        assert root.joinpath(relative).is_file(), relative


def test_vendored_graph_modules_match_manifest_and_ship_licences() -> None:
    root = files("memento.graph_debug").joinpath("static", "vendor")
    manifest = json.loads(root.joinpath("manifest.json").read_text(encoding="utf-8"))
    assert manifest["schema_version"] == 1
    assert [item["name"] for item in manifest["libraries"]] == [
        "three",
        "three-core",
        "preact",
        "preact-hooks",
    ]
    for item in manifest["libraries"]:
        body = root.joinpath(item["file"]).read_bytes()
        assert hashlib.sha256(body).hexdigest() == item["sha256"]
        assert item["version"]
        assert item["license"] == "MIT"
    licences = root.joinpath("LICENSES.md").read_text(encoding="utf-8")
    assert "Three.js 0.180.0" in licences
    assert "Preact 10.27.2" in licences
