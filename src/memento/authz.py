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
    protected_read_prefixes: tuple[str, ...] = ()


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
        protected_read_prefixes=config.protected_read_prefixes,
    )


def require_role(policy: EffectivePolicy, role: str) -> None:
    if role not in policy.roles:
        raise AuthorizationError(f"principal {policy.principal} lacks role: {role}")


def broad_read_grant_warning(
    *,
    roles: tuple[str, ...],
    read_prefixes: tuple[str, ...],
    protected_read_prefixes: tuple[str, ...],
) -> str | None:
    uncovered = any(
        not any(grant == protected for grant in read_prefixes)
        for protected in protected_read_prefixes
    )
    if uncovered and "/" in read_prefixes and "admin" not in roles:
        return (
            "broad '/' read grant excludes protected namespaces; add explicit read prefixes "
            "where access is intended"
        )
    return None


def path_matches_prefix(path: str, prefix: str) -> bool:
    return path == prefix[:-1] or path.startswith(prefix)


def protected_read_grants(
    policy: EffectivePolicy,
) -> tuple[tuple[str, tuple[str, ...]], ...]:
    if "admin" in policy.roles:
        return ()
    return tuple(
        (
            protected,
            tuple(
                grant
                for grant in policy.read_prefixes
                if grant == protected or grant.startswith(protected)
            ),
        )
        for protected in policy.protected_read_prefixes
    )


def authorize_path(policy: EffectivePolicy, path: str, *, action: str) -> AuthorizedNamespace:
    prefixes = policy.read_prefixes if action == "read" else policy.write_prefixes
    if not any(path_matches_prefix(path, prefix) for prefix in prefixes):
        raise AuthorizationError(f"principal {policy.principal} cannot {action} {path}")
    if action == "read":
        for protected, explicit_grants in protected_read_grants(policy):
            if path_matches_prefix(path, protected) and not any(
                path_matches_prefix(path, grant) for grant in explicit_grants
            ):
                raise AuthorizationError(f"principal {policy.principal} cannot read {path}")
    return AuthorizedNamespace(principal=policy.principal, path=path, action=action)


def filter_authorized_paths(policy: EffectivePolicy, paths: list[str], *, action: str) -> list[str]:
    allowed: list[str] = []
    for path in paths:
        try:
            authorize_path(policy, path, action=action)
        except AuthorizationError:
            continue
        allowed.append(path)
    return allowed
