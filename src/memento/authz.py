from __future__ import annotations

from dataclasses import dataclass

from memento.config import AuthorizationConfig, Principal


class AuthorizationError(Exception):
    """Raised when a principal is not allowed to perform an action."""


@dataclass(frozen=True, slots=True)
class AuthorizedNamespace:
    principal: str
    path: str
    action: str


@dataclass(frozen=True, slots=True)
class EffectivePolicy:
    principal: str
    roles: tuple[str, ...]
    read_prefixes: tuple[str, ...]
    write_prefixes: tuple[str, ...]


def resolve_policy(config: AuthorizationConfig, principal: Principal) -> EffectivePolicy:
    try:
        policy = config.principals[principal.name]
    except KeyError as exc:
        raise AuthorizationError(f"unknown principal: {principal.name}") from exc
    missing_roles = set(policy.roles) - set(principal.roles)
    if missing_roles:
        missing = ", ".join(sorted(missing_roles))
        raise AuthorizationError(f"principal {principal.name} is missing required roles: {missing}")
    return EffectivePolicy(
        principal=principal.name,
        roles=policy.roles,
        read_prefixes=policy.read_prefixes,
        write_prefixes=policy.write_prefixes,
    )


def require_role(policy: EffectivePolicy, role: str) -> None:
    if role not in policy.roles:
        raise AuthorizationError(f"principal {policy.principal} lacks role: {role}")


def authorize_path(policy: EffectivePolicy, path: str, *, action: str) -> AuthorizedNamespace:
    prefixes = policy.read_prefixes if action == "read" else policy.write_prefixes
    if any(path == prefix[:-1] or path.startswith(prefix) for prefix in prefixes):
        return AuthorizedNamespace(principal=policy.principal, path=path, action=action)
    raise AuthorizationError(f"principal {policy.principal} cannot {action} {path}")


def filter_authorized_paths(policy: EffectivePolicy, paths: list[str], *, action: str) -> list[str]:
    allowed: list[str] = []
    for path in paths:
        try:
            authorize_path(policy, path, action=action)
        except AuthorizationError:
            continue
        allowed.append(path)
    return allowed
