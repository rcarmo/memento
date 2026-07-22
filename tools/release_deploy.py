from __future__ import annotations

import argparse
import json
import os
import subprocess
import time
import urllib.error
import urllib.parse
import urllib.request
from dataclasses import dataclass
from typing import Any


@dataclass(frozen=True, slots=True)
class HttpResult:
    status: int
    payload: Any


def decode_json_response(text: str) -> Any:
    if not text:
        return None
    try:
        return json.loads(text)
    except json.JSONDecodeError:
        lines = [line for line in text.splitlines() if line.strip()]
        if lines and all(line.lstrip().startswith("{") for line in lines):
            return [json.loads(line) for line in lines]
        raise


def request_json(
    method: str,
    url: str,
    *,
    token: str | None = None,
    data: object | None = None,
    timeout: float = 30.0,
    insecure: bool = False,
    extra_headers: dict[str, str] | None = None,
) -> HttpResult:
    import ssl

    body = None if data is None else json.dumps(data).encode("utf-8")
    headers = {"Accept": "application/json"}
    if body is not None:
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = f"Bearer {token}"
    if extra_headers:
        headers.update(extra_headers)
    req = urllib.request.Request(url, data=body, method=method, headers=headers)
    context = ssl._create_unverified_context() if insecure else None  # noqa: S323 - local ops target
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=context) as response:
            raw = response.read()
            text = raw.decode("utf-8", errors="replace")
            payload = decode_json_response(text)
            return HttpResult(response.status, payload)
    except urllib.error.HTTPError as exc:
        raw = exc.read()
        text = raw.decode("utf-8", errors="replace")
        try:
            error_payload: Any = decode_json_response(text) if text else {"error": exc.reason}
        except json.JSONDecodeError:
            error_payload = {"error": text or exc.reason}
        return HttpResult(exc.code, error_payload)


def keychain_secret(name: str) -> str:
    command = os.environ.get("PICLAW", "piclaw")
    completed = subprocess.run(
        [command, "keychain", "get", name],
        check=True,
        capture_output=True,
        text=True,
        timeout=30,
    )
    start = completed.stdout.rfind("\n{")
    if start >= 0:
        start += 1
    else:
        start = completed.stdout.find("{")
    if start < 0:
        raise SystemExit(f"keychain entry {name} did not return JSON")
    payload = json.loads(completed.stdout[start:])
    secret = payload.get("secret")
    if not isinstance(secret, str) or not secret:
        raise SystemExit(f"keychain entry {name} has no secret")
    return secret


def github_token() -> str:
    return (
        os.environ.get("GITHUB_TOKEN")
        or os.environ.get("GH_TOKEN")
        or keychain_secret("github/piclaw-bot")
    )


def portainer_token() -> str:
    return (
        os.environ.get("PORTAINER_TOKEN")
        or os.environ.get("PORTAINER_RELAY")
        or keychain_secret("portainer/relay")
    )


def wait_release(args: argparse.Namespace) -> None:
    tag = args.tag
    token = github_token()
    headers = {"Authorization": f"Bearer {token}"}
    run: dict[str, Any] | None = None
    deadline = time.monotonic() + args.timeout
    while time.monotonic() < deadline and run is None:
        url = "https://api.github.com/repos/rcarmo/memento/actions/runs?" + urllib.parse.urlencode(
            {"per_page": 20}
        )
        req = urllib.request.Request(
            url,
            headers={
                **headers,
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        with urllib.request.urlopen(req, timeout=30) as response:
            payload = json.loads(response.read().decode("utf-8"))
        runs = [
            item
            for item in payload.get("workflow_runs", [])
            if item.get("name") == "Release" and item.get("head_branch") == tag
        ]
        if runs:
            run = runs[0]
            break
        print(f"release {tag}: waiting for run to appear", flush=True)
        time.sleep(args.interval)
    if run is None:
        raise SystemExit(f"release {tag}: no workflow run before timeout")

    run_id = run["id"]
    print(f"release {tag}: run {run_id} {run['html_url']}", flush=True)
    while time.monotonic() < deadline:
        url = f"https://api.github.com/repos/rcarmo/memento/actions/runs/{run_id}"
        req = urllib.request.Request(
            url,
            headers={
                **headers,
                "Accept": "application/vnd.github+json",
                "X-GitHub-Api-Version": "2022-11-28",
            },
        )
        with urllib.request.urlopen(req, timeout=30) as response:
            run = json.loads(response.read().decode("utf-8"))
        print(f"release {tag}: {run['status']} {run.get('conclusion')}", flush=True)
        if run["status"] == "completed":
            if run.get("conclusion") != "success":
                raise SystemExit(f"release {tag}: failed with {run.get('conclusion')}")
            return
        time.sleep(args.interval)
    raise SystemExit(f"release {tag}: timed out")


def portainer_base() -> str:
    return os.environ.get("PORTAINER_URL", "https://ops.local:9443").rstrip("/")


def portainer_request(
    method: str, path: str, *, data: object | None = None, timeout: float = 60.0
) -> Any:
    result = request_json(
        method,
        f"{portainer_base()}{path}",
        data=data,
        timeout=timeout,
        insecure=True,
        extra_headers={"X-API-Key": portainer_token()},
    )
    if result.status >= 400:
        raise SystemExit(f"Portainer {method} {path} failed: {result.status} {result.payload}")
    return result.payload


def update_config(args: argparse.Namespace) -> None:
    create = portainer_request(
        "POST",
        "/api/endpoints/18/docker/containers/create?"
        + urllib.parse.urlencode({"name": "memento-config-update-make"}),
        data={
            "Image": f"ghcr.io/rcarmo/memento:{args.version}",
            "Entrypoint": ["python"],
            "Cmd": [
                "-c",
                "import json, pathlib\n"
                "path=pathlib.Path('/config/config.json')\n"
                "data=json.loads(path.read_text())\n"
                "semantic=data.setdefault('intelligent_tiers',{}).setdefault('semantic_search',{})\n"
                "semantic.update({'enabled': True, 'worker_mode': 'subprocess', 'worker_path': '/usr/local/bin/memento-embed', 'model_path': '/usr/local/share/memento/models/gte-small.gtemodel', 'ffi_library_path': '/usr/local/lib/memento/libmemento_ffi.so', 'sqlite_extension_path': '/usr/local/lib/memento/libmemento_sqlite_vector.so', 'default_search_mode': 'lexical', 'refresh_on_startup': False})\n"
                "path.write_text(json.dumps(data, indent=2, sort_keys=False)+'\\n')\n"
                "print('updated memento config')\n",
            ],
            "HostConfig": {
                "Binds": ["/volume1/docker/memento/config:/config"],
                "AutoRemove": True,
            },
        },
        timeout=60,
    )
    container_id = create["Id"]
    portainer_request(
        "POST", f"/api/endpoints/18/docker/containers/{container_id}/start", timeout=60
    )
    print("config update helper started", container_id[:12], flush=True)


def compose(version: str) -> str:
    return f"""services:\n  memento:\n    image: ghcr.io/rcarmo/memento:{version}\n    container_name: memento\n    restart: unless-stopped\n    user: "65532:65532"\n    read_only: true\n    init: true\n    entrypoint: ["/bin/sh","-ec"]\n    command:\n      - |\n        set -a\n        . /run/secrets/memento.env\n        set +a\n        exec memento-serve --config /etc/memento/config.json serve --host 0.0.0.0 --port 8000 --endpoint /mcp\n    ports:\n      - "18081:8000"\n    volumes:\n      - /volume1/docker/memento/config/config.json:/etc/memento/config.json:ro\n      - /volume1/docker/memento/config/memento.env:/run/secrets/memento.env:ro\n      - /volume1/docker/memento/state:/var/lib/memento\n    tmpfs:\n      - /tmp:size=32m,mode=1777\n    mem_limit: 512m\n    mem_reservation: 256m\n    pids_limit: 128\n    security_opt:\n      - no-new-privileges:true\n    cap_drop:\n      - ALL\n    healthcheck:\n      test: ["CMD","python","-c","import socket; socket.create_connection(('127.0.0.1',8000),2).close()"]\n      interval: 30s\n      timeout: 5s\n      start_period: 60s\n      retries: 3"""


def deploy(args: argparse.Namespace) -> None:
    portainer_request(
        "POST",
        "/api/endpoints/18/docker/images/create?"
        + urllib.parse.urlencode({"fromImage": "ghcr.io/rcarmo/memento", "tag": args.version}),
        timeout=args.pull_timeout,
    )
    print(f"pulled ghcr.io/rcarmo/memento:{args.version}", flush=True)
    update_config(args)
    try:
        portainer_request(
            "PUT",
            "/api/stacks/111?" + urllib.parse.urlencode({"endpointId": "18"}),
            data={
                "StackFileContent": compose(args.version),
                "Env": [],
                "Prune": True,
                "PullImage": False,
            },
            timeout=args.deploy_timeout,
        )
    except SystemExit as exc:
        print(f"stack update returned before completion: {exc}", flush=True)
    print("stack update requested", flush=True)


def verify(args: argparse.Namespace) -> None:
    base = args.base_url.rstrip("/")
    deadline = time.monotonic() + args.timeout
    while True:
        try:
            result = request_json("GET", f"{base}/graph/api/v1/overview", timeout=20)
            if result.status == 200:
                overview = result.payload
                break
        except Exception as exc:  # noqa: BLE001 - verification reports transient startup failures
            print(f"waiting for graph: {exc}", flush=True)
        if time.monotonic() >= deadline:
            raise SystemExit("graph verification timed out")
        time.sleep(args.interval)
    clusters = overview.get("clusters", [])
    if overview.get("mode") != "aggregated" or not clusters:
        raise SystemExit(f"unexpected overview: {overview.get('mode')} clusters={len(clusters)}")
    skill = next((item for item in clusters if item.get("namespace") == "/skills/"), None)
    if not skill:
        raise SystemExit("missing /skills/ cluster")
    cluster_path = urllib.parse.quote(skill["id"], safe="")
    expanded = request_json(
        "GET", f"{base}/graph/api/v1/clusters/{cluster_path}", timeout=20
    ).payload
    if len(expanded.get("nodes", [])) != 26:
        raise SystemExit(f"unexpected skill node count: {len(expanded.get('nodes', []))}")
    if not all(node.get("tags") for node in expanded.get("nodes", [])):
        raise SystemExit("expanded skill nodes are missing tags")
    print(
        json.dumps(
            {
                "mode": overview["mode"],
                "clusters": len(clusters),
                "skills": len(expanded["nodes"]),
                "all_skill_nodes_have_tags": True,
                "revisions": overview.get("revisions"),
            },
            indent=2,
        )
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="command", required=True)
    wait = sub.add_parser("wait-release")
    wait.add_argument("tag")
    wait.add_argument("--timeout", type=float, default=3600)
    wait.add_argument("--interval", type=float, default=30)
    wait.set_defaults(func=wait_release)

    dep = sub.add_parser("deploy")
    dep.add_argument("version")
    dep.add_argument("--pull-timeout", type=float, default=180)
    dep.add_argument("--deploy-timeout", type=float, default=180)
    dep.set_defaults(func=deploy)

    ver = sub.add_parser("verify")
    ver.add_argument("--base-url", default="http://192.168.1.250:18081")
    ver.add_argument("--timeout", type=float, default=180)
    ver.add_argument("--interval", type=float, default=5)
    ver.set_defaults(func=verify)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()
