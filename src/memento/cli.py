from __future__ import annotations

import argparse
import asyncio
import json
import signal
from collections.abc import Sequence
from contextlib import suppress
from pathlib import Path
from typing import Any

from memento.app import MementoRuntime, build_runtime
from memento.backup import create_backup, restore_backup
from memento.logging import JsonLogger
from memento.metrics import render_prometheus_text
from memento.repository.bundle import audit_repository
from memento.server import MementoMCPServer


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description="Operate the Memento MCP service")
    parser.add_argument("--config", required=True)
    subparsers = parser.add_subparsers(dest="command", required=True)

    serve = subparsers.add_parser("serve", help="Run the MCP server")
    serve.add_argument("--host", default="127.0.0.1")
    serve.add_argument("--port", type=int, default=8000)
    serve.add_argument("--endpoint", default="/mcp")

    audit = subparsers.add_parser("audit", help="Audit the materialized repository")
    audit.add_argument("--path")

    subparsers.add_parser("rebuild-index", help="Rebuild the derived index from Git")

    backup = subparsers.add_parser("backup", help="Create a local backup set")
    backup.add_argument("--output", required=True)

    restore = subparsers.add_parser("restore", help="Restore a backup set")
    restore.add_argument("--input", required=True)
    restore.add_argument("--no-rebuild-derived", action="store_true")

    status = subparsers.add_parser("status", help="Show operational status")
    status.add_argument("--format", choices=("json", "prometheus"), default="json")

    dream = subparsers.add_parser("dream", help="Run Dream scanner/proposal mode once")
    dream.add_argument("--mode", choices=("disabled", "report_only", "propose"))
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    logger = JsonLogger()
    config_path = Path(args.config)
    if args.command == "restore":
        from memento.app import load_service_config

        payload = restore_backup(
            load_service_config(config_path),
            Path(args.input),
            rebuild_derived=not args.no_rebuild_derived,
        )
        _emit_json(payload)
        logger.info("restore_completed", backup_dir=args.input, **payload)
        return 0

    runtime = build_runtime(config_path)
    try:
        if args.command == "serve":
            logger.info("serve_starting", host=args.host, port=args.port, endpoint=args.endpoint)
            return asyncio.run(
                _serve(
                    runtime,
                    host=args.host,
                    port=args.port,
                    endpoint=args.endpoint,
                    logger=logger,
                )
            )
        if args.command == "audit":
            payload = _audit(runtime, path=args.path)
        elif args.command == "rebuild-index":
            payload = runtime.rebuild_index()
        elif args.command == "backup":
            payload = create_backup(runtime, Path(args.output)).as_dict()
        elif args.command == "status":
            payload = runtime.status_snapshot()
            if args.format == "prometheus":
                print(render_prometheus_text(runtime), end="")
                return 0
        elif args.command == "dream":
            payload = runtime.service.run_dream(mode=args.mode)
        else:  # pragma: no cover
            raise AssertionError(f"unsupported command {args.command}")
        _emit_json(payload)
        logger.info("command_completed", command=args.command, result=payload)
        return 0
    finally:
        runtime.close()
        logger.info("runtime_closed", command=args.command)


async def _serve(
    runtime: MementoRuntime,
    *,
    host: str,
    port: int,
    endpoint: str,
    logger: JsonLogger,
) -> int:
    server = runtime.build_server()
    shutdown_event = asyncio.Event()
    install_signal_handlers(shutdown_event, logger)
    serve_task = asyncio.create_task(run_server(server, host=host, port=port, endpoint=endpoint))
    stop_task = asyncio.create_task(shutdown_event.wait())
    done, pending = await asyncio.wait({serve_task, stop_task}, return_when=asyncio.FIRST_COMPLETED)
    if stop_task in done:
        logger.info("shutdown_requested", host=host, port=port)
        await drain_server(server)
        serve_task.cancel()
        with suppress(asyncio.CancelledError):
            await serve_task
        return 0
    stop_task.cancel()
    with suppress(asyncio.CancelledError):
        await stop_task
    result = await serve_task
    return int(result or 0)


def install_signal_handlers(shutdown_event: asyncio.Event, logger: JsonLogger) -> None:
    loop = asyncio.get_running_loop()
    for signame in (signal.SIGINT, signal.SIGTERM):
        with suppress(NotImplementedError):
            loop.add_signal_handler(
                signame,
                _shutdown_callback(signame, shutdown_event, logger),
            )


def _shutdown_callback(
    signame: signal.Signals,
    shutdown_event: asyncio.Event,
    logger: JsonLogger,
) -> Any:
    def callback() -> None:
        _request_shutdown(signame, shutdown_event, logger)

    return callback


def _request_shutdown(
    signame: signal.Signals,
    shutdown_event: asyncio.Event,
    logger: JsonLogger,
) -> None:
    logger.info("signal_received", signal=signame.name)
    shutdown_event.set()


async def run_server(server: MementoMCPServer, *, host: str, port: int, endpoint: str) -> Any:
    return await server.run_streamable_http_async(host=host, port=port, endpoint=endpoint)


async def drain_server(server: object) -> None:
    for name in ("shutdown", "aclose", "close"):
        candidate = getattr(server, name, None)
        if candidate is None:
            continue
        result = candidate()
        if asyncio.iscoroutine(result):
            await result
        return


def _audit(runtime: MementoRuntime, *, path: str | None) -> dict[str, Any]:
    audit = audit_repository(runtime.paths.repo_paths.current_dir)
    issues = [issue.__dict__ for issue in audit.issues if path is None or issue.bundle_path == path]
    return {"ok": not issues, "issues": issues}


def _emit_json(payload: dict[str, Any]) -> None:
    print(json.dumps(payload, indent=2, sort_keys=True))


if __name__ == "__main__":  # pragma: no cover
    raise SystemExit(main())
