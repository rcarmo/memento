from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

from memento.repository.frontmatter import FrontmatterError, parse_concept_file, serialize_concept
from memento.repository.links import extract_structural_links
from memento.repository.paths import is_reserved_bundle_path, validate_repository_read_path
from memento.repository.schema import ConceptDocument


class BundleError(Exception):
    """Raised when bundle scanning or auditing fails."""


@dataclass(frozen=True, slots=True)
class BundleEntry:
    bundle_path: str
    document: ConceptDocument


@dataclass(frozen=True, slots=True)
class AuditIssue:
    bundle_path: str
    code: str
    message: str


@dataclass(frozen=True, slots=True)
class RepositoryAudit:
    issues: tuple[AuditIssue, ...]
    generated_indexes: dict[str, str]
    generated_log: str

    @property
    def ok(self) -> bool:
        return not self.issues


@dataclass(frozen=True, slots=True)
class RepositoryBundle:
    root: Path
    entries: tuple[BundleEntry, ...]

    def get(self, bundle_path: str) -> BundleEntry:
        for entry in self.entries:
            if entry.bundle_path == bundle_path:
                return entry
        raise BundleError(f"unknown bundle path: {bundle_path}")


def scan_bundle(root: Path) -> RepositoryBundle:
    entries: list[BundleEntry] = []
    for path in sorted(root.rglob("*.md")):
        bundle_path = "/" + path.relative_to(root).as_posix()
        if is_reserved_bundle_path(bundle_path):
            continue
        validate_repository_read_path(root, bundle_path)
        try:
            document = parse_concept_file(path)
        except FrontmatterError as exc:
            raise BundleError(f"invalid concept at {bundle_path}") from exc
        entries.append(BundleEntry(bundle_path=bundle_path, document=document))
    return RepositoryBundle(root=root, entries=tuple(entries))


def read_bundle_entry(root: Path, bundle_path: str) -> BundleEntry:
    safe_path = validate_repository_read_path(root, bundle_path)
    document = parse_concept_file(safe_path.absolute_path)
    return BundleEntry(bundle_path=bundle_path, document=document)


def generate_directory_indexes(bundle: RepositoryBundle) -> dict[str, str]:
    directories = {"/"}
    for entry in bundle.entries:
        path = Path(entry.bundle_path.removeprefix("/"))
        prefixes = ["/"]
        for index in range(1, len(path.parts)):
            prefixes.append("/" + "/".join(path.parts[:index]) + "/")
        directories.update(prefixes)
    generated: dict[str, str] = {}
    for directory in sorted(directories):
        generated[directory] = _render_directory_index(bundle, directory)
    return generated


def generate_root_log(bundle: RepositoryBundle) -> str:
    lines = [
        "# Mutation Log",
        "",
        "This file is generated deterministically from concept metadata.",
        "",
    ]
    sorted_entries = sorted(
        bundle.entries,
        key=lambda entry: (
            entry.document.frontmatter.updated_at,
            entry.document.frontmatter.title.casefold(),
            entry.bundle_path,
        ),
        reverse=True,
    )
    for entry in sorted_entries:
        frontmatter = entry.document.frontmatter
        updated_at = frontmatter.updated_at.isoformat().replace("+00:00", "Z")
        lines.append(
            f"- {updated_at} — [{frontmatter.title}]({entry.bundle_path})"
            f" — {frontmatter.updated_by}"
        )
    return "\n".join(lines) + "\n"


def audit_repository(root: Path) -> RepositoryAudit:
    issues: list[AuditIssue] = []
    bundle = scan_bundle(root)
    seen_ids: dict[str, str] = {}
    for entry in bundle.entries:
        concept_id = entry.document.frontmatter.id
        previous_path = seen_ids.get(concept_id)
        if previous_path is not None:
            issues.append(
                AuditIssue(
                    bundle_path=entry.bundle_path,
                    code="duplicate_id",
                    message=f"concept id {concept_id} already used by {previous_path}",
                )
            )
        else:
            seen_ids[concept_id] = entry.bundle_path
        links = extract_structural_links(entry.document.body)
        for link in links:
            if not link.href.startswith("/"):
                continue
            target_path = link.href.split("#", 1)[0]
            if is_reserved_bundle_path(target_path):
                continue
            if target_path not in {item.bundle_path for item in bundle.entries}:
                issues.append(
                    AuditIssue(
                        bundle_path=entry.bundle_path,
                        code="broken_link",
                        message=f"broken link to {target_path}",
                    )
                )
        serialized = serialize_concept(entry.document)
        if not serialized.endswith("\n"):
            issues.append(
                AuditIssue(
                    bundle_path=entry.bundle_path,
                    code="serialization",
                    message="serialized concept must end with newline",
                )
            )
    return RepositoryAudit(
        issues=tuple(issues),
        generated_indexes=generate_directory_indexes(bundle),
        generated_log=generate_root_log(bundle),
    )


def _render_directory_index(bundle: RepositoryBundle, directory: str) -> str:
    lines = ["# Index", ""]
    children = sorted(_collect_directory_children(bundle, directory), key=str.casefold)
    if not children:
        lines.append("_Empty directory._")
        return "\n".join(lines) + "\n"
    for child in children:
        lines.append(f"- {child}")
    return "\n".join(lines) + "\n"


def _collect_directory_children(bundle: RepositoryBundle, directory: str) -> list[str]:
    results: set[str] = set()
    prefix = directory.removeprefix("/")
    for entry in bundle.entries:
        entry_path = entry.bundle_path.removeprefix("/")
        if prefix:
            prefix_without_slash = prefix.rstrip("/") + "/"
            if not entry_path.startswith(prefix_without_slash):
                continue
            remainder = entry_path.removeprefix(prefix_without_slash)
        else:
            remainder = entry_path
        first_part = remainder.split("/", 1)[0]
        if "/" in remainder:
            child_directory = "/".join(filter(None, [prefix.rstrip("/"), first_part]))
            results.add(f"[{first_part}](/{child_directory}/index.md)")
            continue
        title = bundle.get("/" + entry_path).document.frontmatter.title
        results.add(f"[{title}](/{entry_path})")
    return list(results)
