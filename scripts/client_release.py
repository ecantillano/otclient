#!/usr/bin/env python3
"""Deterministic packaging and release validation for the Thappy client.

This module intentionally uses only the Python standard library.  Linux
``tar.zst`` support delegates compression/decompression to the system zstd
binary, which is installed explicitly by the release workflows.
"""

from __future__ import annotations

import dataclasses
import datetime as dt
import hashlib
import json
import os
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile
import urllib.parse
import zipfile
from collections import Counter, defaultdict
from pathlib import Path, PurePosixPath
from typing import Dict, Iterable, Iterator, List, Mapping, Optional, Sequence, Tuple


SCHEMA_VERSION = 1
MINIMUM_LAUNCHER_VERSION = "0.1.0"
SEMVER_RE = re.compile(r"^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-([0-9A-Za-z.-]+))?(?:\+[0-9A-Za-z.-]+)?$")
CANONICAL_LOGIN_URL = "https://login.thappy.cl/login.php"
CANONICAL_STABLE_MANIFEST_URL = "https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json"
CANONICAL_CONFIG_PATTERNS = (
    re.compile(r"port\s*=\s*443(?:\D|$)"),
    re.compile(r"protocol\s*=\s*1525(?:\D|$)"),
    re.compile(r"httpLogin\s*=\s*false\b"),
    re.compile(r"useAuthenticator\s*=\s*false\b"),
)

PLATFORM_EXECUTABLES = {"windows": "Thappy.exe", "linux": "thappy"}
PLATFORM_LAUNCHERS = {"windows": "ThappyLauncher.exe", "linux": "thappy-launcher"}
RUNTIME_EXECUTABLES = frozenset(PLATFORM_EXECUTABLES.values()) | frozenset(PLATFORM_LAUNCHERS.values())
PLATFORM_LABELS = {"windows": "Windows-x86_64", "linux": "Linux-x86_64"}

CORE_MODULES = {
    "client",
    "corelib",
    "gamelib",
    "modulelib",
    "startup",
}

# UI/features that do not belong in the retro Thappy distribution.  Protocol
# handling stays in C++; this list removes only runtime UI/resource modules.
MODERN_MODULES = {
    "client_bottommenu",
    "client_debug_info",
    "client_serverlist",
    "client_terminal",
    "game_actionbar",
    "game_analyser",
    "game_attachedeffects",
    "game_blessing",
    "game_creatureinformation",
    "game_cyclopedia",
    "game_forge",
    "game_healthcircle",
    "game_highscore",
    "game_htmlsample",
    "game_imbuementtracker",
    "game_imbuing",
    "game_inspect",
    "game_lootsplitter",
    "game_market",
    "game_notifications",
    "game_paperdolls",
    "game_playermount",
    "game_prey",
    "game_proficiency",
    "game_quickloot",
    "game_rewardwall",
    "game_shop",
    "game_stash",
    "game_store",
    "game_taskboard",
    "game_tutorial",
    "game_wheel",
    "updater",
}

FORBIDDEN_DIRECTORY_NAMES = {
    ".git",
    ".github",
    "build",
    "build-release",
    "cmakefiles",
    "debug",
    "docs",
    "include",
    "out",
    "records",
    "src",
    "source",
    "tests",
    "tools",
    "vcpkg",
    "vcpkg_installed",
}
FORBIDDEN_SUFFIXES = {
    ".a",
    ".cmake",
    ".cpp",
    ".cxx",
    ".cc",
    ".exp",
    ".h",
    ".hpp",
    ".ilk",
    ".lib",
    ".o",
    ".obj",
    ".pdb",
    ".psd",
    ".pyc",
    ".sln",
    ".vcxproj",
    ".xcf",
    ".blend",
    ".kra",
}
TEXT_SUFFIXES = {
    ".cfg",
    ".css",
    ".html",
    ".ini",
    ".json",
    ".lua",
    ".md",
    ".otfont",
    ".otml",
    ".otmod",
    ".otui",
    ".txt",
    ".xml",
}
ASSET_PREFIXES = (
    "data/catalogs/",
    "data/sounds/",
    "data/things/",
)
MODERN_DATA_PREFIXES = (
    "data/images/game/actionbar/",
    "data/images/game/analyzer/",
    "data/images/game/cyclopedia/",
    "data/images/game/healthcircle/",
    "data/images/game/imbuing/",
    "data/images/game/mobile/",
    "data/images/game/prey/",
    "data/images/game/tutorial/",
    "data/images/game/wheel/",
    "data/images/store/",
    "data/images/ui/actionbar/",
)
MODERN_RESOURCE_TERMS = (
    "analyser",
    "analyzer",
    "bot",
    "cyclopedia",
    "forge",
    "imbuement",
    "prey",
    "quickloot",
    "rewardwall",
    "skillwheel",
    "stash",
    "store",
    "taskboard",
    "wheel",
)
ROOT_RUNTIME_FILES = (
    "init.lua",
    "otclientrc.lua",
    "config.ini",
    "cacert.pem",
    "LICENSE",
    "THIRD_PARTY_NOTICES.md",
)
GENERATED_RUNTIME_PATHS = {
    "artifact-environment.json",
    "build-metadata.json",
    "config/build_config.lua",
}
REQUIRED_LICENSE_PATHS = {
    "license",
    "third_party_notices.md",
    "licenses/otclient-mit.txt",
}
PRESERVE_PREFIXES = (
    ".thappy-launcher/",
    "launcher-config.json",
    "ThappyLauncher.exe",
    "thappy-launcher",
    "config.otml",
    "settings/",
    "profiles/",
    "screenshots/",
    "records/",
    "logs/",
    "minimap/",
)
LAUNCHER_PRESERVE_PATHS = (
    ".thappy-launcher/",
    "launcher-config.json",
    "ThappyLauncher.exe",
    "thappy-launcher",
    "config.otml",
    "settings/",
    "profiles/",
    "screenshots/",
    "records/",
    "logs/",
    "minimap/",
)
DEFAULT_DELETE_PATHS = tuple(
    sorted(
        [
            "mods/game_bot/",
            "mods/game_tasks/",
            "otclient",
            "otclient.exe",
            "otclient_dx.exe",
            "otclient.ilk",
            "otclient.pdb",
        ]
        + ["modules/{}/".format(name) for name in MODERN_MODULES]
    )
)

BOT_TEXT_RE = re.compile(
    r"(?i)(?:\bgame_bot\b|\bmodules\.game_bot\b|\bvbot\b|\brvbot\b|\bcavebot\b|"
    r"\btargetbot\b|\bbotserver\b|\bdefault_configs\b|\bautoheal\b|\bautoloot\b|"
    r"\blooter\b|\bbot(?:button|window|menu|panel|icon)\b|/bot/)"
)
PRIVATE_ENDPOINT_RE = re.compile(
    r"(?i)(?:\blocalhost\b|\b127\.\d{1,3}\.\d{1,3}\.\d{1,3}\b|"
    r"\b10\.\d{1,3}\.\d{1,3}\.\d{1,3}\b|"
    r"\b192\.168\.\d{1,3}\.\d{1,3}\b|"
    r"\b172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}\b|"
    r"\b0\.0\.0\.0\b)"
)


class ReleaseError(RuntimeError):
    """A packaging or release-contract violation."""


@dataclasses.dataclass(frozen=True)
class PackageFile:
    source: Path
    archive_path: str
    component: str


@dataclasses.dataclass(frozen=True)
class ArchiveEntry:
    name: str
    size: int
    compressed_size: int
    mode: int
    data: bytes


@dataclasses.dataclass
class AuditResult:
    archive: str
    files: int
    compressed_size: int
    uncompressed_size: int
    errors: List[str]
    warnings: List[str]
    top100: List[dict]
    folders: Dict[str, dict]

    @property
    def ok(self) -> bool:
        return not self.errors

    def as_dict(self) -> dict:
        return {
            "schema_version": SCHEMA_VERSION,
            "archive": self.archive,
            "ok": self.ok,
            "files": self.files,
            "compressed_size": self.compressed_size,
            "uncompressed_size": self.uncompressed_size,
            "errors": self.errors,
            "warnings": self.warnings,
            "folders": self.folders,
            "top100": self.top100,
        }


def validate_version(version: str) -> None:
    if not SEMVER_RE.fullmatch(version):
        raise ReleaseError("version must be SemVer without a leading v: {!r}".format(version))


def validate_channel(channel: str) -> None:
    if channel not in {"stable", "test", "local"}:
        raise ReleaseError("channel must be stable, test, or local")


def validate_platform(platform: str) -> None:
    if platform not in PLATFORM_EXECUTABLES:
        raise ReleaseError("platform must be windows or linux")


def safe_archive_path(value: str) -> str:
    normalized = value.replace("\\", "/")
    pure = PurePosixPath(normalized)
    if not normalized or normalized.startswith("/") or pure.is_absolute():
        raise ReleaseError("unsafe archive path {!r}".format(value))
    if any(part in {"", ".", ".."} for part in pure.parts):
        raise ReleaseError("unsafe archive path {!r}".format(value))
    if re.match(r"^[A-Za-z]:", normalized):
        raise ReleaseError("unsafe archive path {!r}".format(value))
    return pure.as_posix()


def _file_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _file_sha1(path: Path) -> str:
    """Return the SHA-1 required by SPDX package verification codes."""

    digest = hashlib.sha1()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _bytes_sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _iter_regular_files(root: Path) -> Iterator[Path]:
    if not root.exists():
        return
    for path in sorted(root.rglob("*"), key=lambda item: item.as_posix().lower()):
        if path.is_symlink():
            raise ReleaseError("symlink is forbidden in package input: {}".format(path))
        if path.is_file():
            yield path


def _is_forbidden_path(path: str) -> Optional[str]:
    normalized = safe_archive_path(path)
    parts = [part.lower() for part in PurePosixPath(normalized).parts]
    if any(part in FORBIDDEN_DIRECTORY_NAMES for part in parts[:-1]):
        return "development/build directory"
    suffix = PurePosixPath(normalized).suffix.lower()
    if suffix in FORBIDDEN_SUFFIXES:
        return "development/build file extension"
    if "game_bot" in parts or "default_configs" in parts:
        return "BOT content"
    if parts and parts[0] == "mods":
        return "mods are not mounted by the official client"
    if len(parts) >= 2 and parts[0] == "modules" and parts[1] in MODERN_MODULES:
        return "modern UI module"
    if len(parts) >= 2 and parts[0] == "mods" and parts[1] in {"game_bot", "game_tasks"}:
        return "BOT/modern mod"
    if any("offlinetraining" in part or "offline_training" in part for part in parts):
        return "offline training UI"
    if _is_modern_data_resource(normalized):
        return "modern/BOT visual resource"
    return None


def _is_modern_data_resource(path: str) -> bool:
    lower = path.lower()
    if any(lower.startswith(prefix) for prefix in MODERN_DATA_PREFIXES):
        return True
    if lower == "data/cursors/quicklootcursor.png" or lower == "data/json/skillwheelstringsjsonlibrary.json":
        return True
    scoped_roots = (
        "data/images/icons/",
        "data/images/options/",
        "data/images/topbuttons/",
        "data/fonts/otfont/",
    )
    return lower.startswith(scoped_roots) and any(term in PurePosixPath(lower).name for term in MODERN_RESOURCE_TERMS)


def collect_package_files(
    source_root: Path,
    binary: Path,
    launcher_binary: Path,
    platform: str,
    runtime_dir: Optional[Path] = None,
) -> List[PackageFile]:
    """Collect a strict runtime allowlist and assign every file to a component."""

    validate_platform(platform)
    source_root = source_root.resolve()
    binary = binary.resolve()
    launcher_binary = launcher_binary.resolve()
    if not binary.is_file() or binary.is_symlink():
        raise ReleaseError("compiled client binary does not exist or is a symlink: {}".format(binary))
    if not launcher_binary.is_file() or launcher_binary.is_symlink():
        raise ReleaseError("compiled launcher binary does not exist or is a symlink: {}".format(launcher_binary))

    records: List[PackageFile] = []

    def add(path: Path, archive_path: str, component: str) -> None:
        archive_path = safe_archive_path(archive_path)
        reason = _is_forbidden_path(archive_path)
        if reason:
            raise ReleaseError("{} is forbidden in runtime package ({})".format(archive_path, reason))
        records.append(PackageFile(path.resolve(), archive_path, component))

    add(launcher_binary, PLATFORM_LAUNCHERS[platform], "bootstrap")
    add(binary, PLATFORM_EXECUTABLES[platform], "core")
    if runtime_dir is not None:
        runtime_dir = runtime_dir.resolve()
        if not runtime_dir.is_dir():
            raise ReleaseError("runtime library directory does not exist: {}".format(runtime_dir))
        for path in sorted(runtime_dir.iterdir(), key=lambda item: item.name.lower()):
            if not path.is_file() or path.is_symlink() or path.resolve() in {binary, launcher_binary}:
                continue
            lower = path.name.lower()
            if platform == "windows" and lower.endswith(".dll"):
                add(path, path.name, "core")
            elif platform == "linux" and re.search(r"\.so(?:\.\d+(?:\.\d+)*)?$", lower):
                add(path, path.name, "core")

    missing = []
    for relative in ROOT_RUNTIME_FILES:
        path = source_root / relative
        if path.is_file():
            add(path, relative, "core")
        else:
            missing.append(relative)
    required_root = {"init.lua", "cacert.pem", "LICENSE", "THIRD_PARTY_NOTICES.md"}
    missing_required = sorted(required_root.intersection(missing))
    if missing_required:
        raise ReleaseError("missing required runtime files: {}".format(", ".join(missing_required)))

    for tree_name in ("config", "licenses"):
        tree_root = source_root / tree_name
        for path in _iter_regular_files(tree_root):
            relative = path.relative_to(source_root).as_posix()
            if relative.lower() in GENERATED_RUNTIME_PATHS:
                continue
            add(path, relative, "core")

    modules_root = source_root / "modules"
    if not modules_root.is_dir():
        raise ReleaseError("missing modules directory")
    for module_dir in sorted(modules_root.iterdir(), key=lambda item: item.name.lower()):
        if not module_dir.is_dir() or module_dir.is_symlink():
            continue
        module_name = module_dir.name.lower()
        if module_name in MODERN_MODULES:
            continue
        component = "core" if module_name in CORE_MODULES else "modules"
        for path in _iter_regular_files(module_dir):
            relative = path.relative_to(source_root).as_posix()
            if _is_forbidden_path(relative):
                continue
            add(path, relative, component)

    data_root = source_root / "data"
    if not data_root.is_dir():
        raise ReleaseError("missing data directory")
    for path in _iter_regular_files(data_root):
        relative = path.relative_to(source_root).as_posix()
        if _is_forbidden_path(relative):
            continue
        component = "assets" if relative.startswith(ASSET_PREFIXES) else "data"
        if component == "assets" and path.name.lower() in {"readme", "readme.md", ".gitignore", ".gitkeep"}:
            continue
        add(path, relative, component)

    seen: Dict[str, str] = {}
    for record in records:
        key = record.archive_path.lower()
        if key in seen:
            raise ReleaseError("duplicate/case-colliding package path: {} and {}".format(seen[key], record.archive_path))
        seen[key] = record.archive_path
    return sorted(records, key=lambda record: record.archive_path.lower())


def _zip_datetime(epoch: int) -> Tuple[int, int, int, int, int, int]:
    # ZIP cannot represent dates before 1980.
    value = dt.datetime.fromtimestamp(max(epoch, 315532800), tz=dt.timezone.utc)
    return value.year, value.month, value.day, value.hour, value.minute, value.second


def write_zip(path: Path, records: Sequence[PackageFile], epoch: int) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED, compresslevel=9, allowZip64=True) as archive:
        for record in sorted(records, key=lambda item: item.archive_path.lower()):
            info = zipfile.ZipInfo(record.archive_path, _zip_datetime(epoch))
            executable = record.archive_path in RUNTIME_EXECUTABLES
            info.external_attr = ((0o755 if executable else 0o644) & 0xFFFF) << 16
            info.compress_type = zipfile.ZIP_DEFLATED
            info.create_system = 3
            with record.source.open("rb") as source, archive.open(info, "w", force_zip64=True) as destination:
                shutil.copyfileobj(source, destination, length=1024 * 1024)


def _require_zstd() -> str:
    executable = shutil.which("zstd")
    if not executable:
        raise ReleaseError("zstd executable is required for Linux tar.zst packages")
    return executable


def write_tar_zst(path: Path, records: Sequence[PackageFile], epoch: int) -> None:
    zstd = _require_zstd()
    path.parent.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="thappy-tar-") as temporary:
        tar_path = Path(temporary) / "client.tar"
        with tarfile.open(str(tar_path), "w", format=tarfile.PAX_FORMAT) as archive:
            for record in sorted(records, key=lambda item: item.archive_path.lower()):
                info = archive.gettarinfo(str(record.source), arcname=record.archive_path)
                info.uid = 0
                info.gid = 0
                info.uname = "root"
                info.gname = "root"
                info.mtime = epoch
                info.mode = 0o755 if record.archive_path in RUNTIME_EXECUTABLES else 0o644
                with record.source.open("rb") as handle:
                    archive.addfile(info, handle)
        subprocess.run(
            [zstd, "-q", "-f", "-19", "-T1", str(tar_path), "-o", str(path)],
            check=True,
        )


def _validate_entry_name(name: str, seen: Dict[str, str]) -> str:
    normalized = safe_archive_path(name.rstrip("/"))
    key = normalized.lower()
    if key in seen:
        raise ReleaseError("duplicate/case-colliding archive path: {} and {}".format(seen[key], normalized))
    seen[key] = normalized
    return normalized


def _read_zip(path: Path) -> List[ArchiveEntry]:
    entries: List[ArchiveEntry] = []
    seen: Dict[str, str] = {}
    with zipfile.ZipFile(path) as archive:
        for info in archive.infolist():
            if info.is_dir():
                continue
            mode = (info.external_attr >> 16) & 0xFFFF
            if stat.S_ISLNK(mode):
                raise ReleaseError("symlink is forbidden in archive: {}".format(info.filename))
            name = _validate_entry_name(info.filename, seen)
            if info.file_size > 2 * 1024 * 1024 * 1024:
                raise ReleaseError("archive entry exceeds 2 GiB: {}".format(name))
            with archive.open(info) as handle:
                data = handle.read()
            if len(data) != info.file_size:
                raise ReleaseError("archive entry size mismatch: {}".format(name))
            entries.append(ArchiveEntry(name, info.file_size, info.compress_size, mode, data))
    return entries


def _read_tar_zst(path: Path) -> List[ArchiveEntry]:
    zstd = _require_zstd()
    entries: List[ArchiveEntry] = []
    seen: Dict[str, str] = {}
    with tempfile.TemporaryDirectory(prefix="thappy-untar-") as temporary:
        tar_path = Path(temporary) / "client.tar"
        subprocess.run([zstd, "-q", "-d", "-f", str(path), "-o", str(tar_path)], check=True)
        with tarfile.open(str(tar_path), "r:") as archive:
            members = archive.getmembers()
            file_members = [member for member in members if member.isfile()]
            if any(not (member.isfile() or member.isdir()) for member in members):
                raise ReleaseError("tar.zst contains links or special files")
            total_size = sum(member.size for member in file_members)
            compressed_total = path.stat().st_size
            for member in file_members:
                name = _validate_entry_name(member.name, seen)
                handle = archive.extractfile(member)
                if handle is None:
                    raise ReleaseError("cannot read tar entry {}".format(name))
                data = handle.read()
                estimate = int(compressed_total * member.size / total_size) if total_size else 0
                entries.append(ArchiveEntry(name, member.size, estimate, member.mode, data))
    return entries


def read_archive(path: Path) -> List[ArchiveEntry]:
    path = path.resolve()
    if path.name.lower().endswith(".tar.zst"):
        return _read_tar_zst(path)
    if path.suffix.lower() == ".zip":
        return _read_zip(path)
    raise ReleaseError("unsupported package format: {}".format(path.name))


def _decode_text(entry: ArchiveEntry) -> Optional[str]:
    suffix = PurePosixPath(entry.name).suffix.lower()
    if suffix not in TEXT_SUFFIXES and PurePosixPath(entry.name).name.lower() not in {"license", "init.lua", "otclientrc.lua"}:
        return None
    if b"\x00" in entry.data[:4096]:
        return None
    try:
        return entry.data.decode("utf-8")
    except UnicodeDecodeError:
        return entry.data.decode("utf-8", errors="replace")


def _has_debug_symbols(entry: ArchiveEntry) -> Optional[str]:
    lower = entry.name.lower()
    suffix = PurePosixPath(lower).suffix
    if suffix in {".pdb", ".ilk", ".exp", ".lib", ".obj", ".o", ".a"}:
        return "debug/build artifact extension"
    if entry.data.startswith(b"\x7fELF"):
        markers = (b".debug_info", b".debug_line", b".debug_str", b".symtab")
        if any(marker in entry.data for marker in markers):
            return "ELF contains debug or full symbol table sections"
    if entry.data.startswith(b"MZ") and (b".pdb\x00" in entry.data.lower() or b"RSDS" in entry.data):
        return "PE contains a PDB/debug directory reference"
    return None


def is_safe_bot_purge_migration(path: str, text: str) -> bool:
    """Allow BOT identifiers only in the exact legacy-key purge migration.

    The exception is intentionally semantic: BOT identifiers must be confined
    to the removal-key table/loop, a removal call is mandatory, and gameplay,
    scheduling, code-loading, process, or network APIs are forbidden.
    """

    if path.lower() != "config/settings_migration.lua":
        return False
    if "removedBotKeys" not in text or not re.search(r"settings\.remove\s*\(\s*key\s*\)", text):
        return False
    dangerous = re.compile(
        r"(?i)(?:g_game|scheduleEvent|cycleEvent|macro\s*\(|attack\s*\(|autoWalk|findPath|"
        r"loadstring|loadfile|dofile|require\s*\(|g_modules|g_http|http\.|https?://|socket|"
        r"os\.|io\.|send\w*\s*\(|connect\w*\s*\()"
    )
    if dangerous.search(text):
        return False
    for line in text.splitlines():
        if not BOT_TEXT_RE.search(line):
            continue
        stripped = line.strip()
        allowed_key = bool(re.fullmatch(r"['\"][A-Za-z0-9_-]+['\"],?", stripped))
        allowed_structure = "removedBotKeys" in stripped or "settings.remove" in stripped
        if not (allowed_key or allowed_structure or stripped.startswith("--")):
            return False
    return True


def is_safe_launcher_purge_config(path: str, text: str) -> bool:
    """Allow BOT names only as exact local deletion rules in launcher config."""

    if path.lower() != "launcher-config.json":
        return False
    try:
        payload = json.loads(text)
    except (TypeError, ValueError):
        return False
    if not isinstance(payload, dict) or not isinstance(payload.get("delete_allowlist"), list):
        return False
    without_deletes = dict(payload)
    delete_rules = without_deletes.pop("delete_allowlist")
    if BOT_TEXT_RE.search(json.dumps(without_deletes, sort_keys=True)):
        return False
    for rule in delete_rules:
        if not isinstance(rule, str):
            return False
        if BOT_TEXT_RE.search(rule) and rule.lower() not in {"mods/game_bot/"}:
            return False
    return True


def audit_entries(
    entries: Sequence[ArchiveEntry],
    archive_name: str,
    compressed_size: int,
    require_layout: bool = True,
    require_production_config: bool = True,
    allow_private_endpoints: bool = False,
) -> AuditResult:
    errors: List[str] = []
    warnings: List[str] = []
    paths = {entry.name.lower() for entry in entries}
    text_entries: List[Tuple[ArchiveEntry, str]] = []
    library_names: Dict[str, List[ArchiveEntry]] = defaultdict(list)

    for entry in entries:
        reason = _is_forbidden_path(entry.name)
        if reason:
            errors.append("forbidden path {} ({})".format(entry.name, reason))
        debug_reason = _has_debug_symbols(entry)
        if debug_reason:
            errors.append("{}: {}".format(entry.name, debug_reason))
        lower = entry.name.lower()
        if re.search(r"(?:\.dll|\.so(?:\.\d+(?:\.\d+)*)?)$", lower):
            library_names[PurePosixPath(lower).name].append(entry)
        text = _decode_text(entry)
        if text is not None:
            text_entries.append((entry, text))
            if BOT_TEXT_RE.search(text) and not (
                is_safe_bot_purge_migration(entry.name, text)
                or is_safe_launcher_purge_config(entry.name, text)
            ):
                errors.append("{} contains BOT reference".format(entry.name))
            private = PRIVATE_ENDPOINT_RE.search(text)
            if private and not allow_private_endpoints:
                errors.append("{} contains private/development endpoint {!r}".format(entry.name, private.group(0)))

    for basename, duplicates in sorted(library_names.items()):
        if len(duplicates) > 1:
            hashes = {_bytes_sha256(entry.data) for entry in duplicates}
            paths_text = ", ".join(entry.name for entry in duplicates)
            errors.append(
                "duplicate runtime library basename {} at {} ({})".format(
                    basename,
                    paths_text,
                    "identical" if len(hashes) == 1 else "different content",
                )
            )

    if require_layout:
        executable_present = any(path in paths for path in ("thappy.exe", "thappy"))
        if not executable_present:
            errors.append("package is missing Thappy.exe/thappy")
        launcher_present = any(path in paths for path in ("thappylauncher.exe", "thappy-launcher"))
        if not launcher_present:
            errors.append("package is missing ThappyLauncher.exe/thappy-launcher")
        if not any(path.startswith("data/") for path in paths):
            errors.append("package is missing data/")
        if not any(path.startswith("modules/") for path in paths):
            errors.append("package is missing modules/")
        missing_licenses = sorted(REQUIRED_LICENSE_PATHS - paths)
        if missing_licenses:
            errors.append("package is missing required notices: {}".format(", ".join(missing_licenses)))

    if require_production_config:
        canonical_files = []
        for entry, text in text_entries:
            if CANONICAL_LOGIN_URL in text:
                canonical_files.append((entry, text))
        if not canonical_files:
            errors.append("production package does not contain the canonical Thappy login URL")
        elif not any(all(pattern.search(text) for pattern in CANONICAL_CONFIG_PATTERNS) for _, text in canonical_files):
            errors.append("canonical production server entry has wrong port/protocol/httpLogin/authenticator values")
        if not any(CANONICAL_STABLE_MANIFEST_URL in text for _, text in text_entries):
            errors.append("production package does not contain the canonical stable manifest URL")
        for entry, text in canonical_files:
            if re.search(r"protocol\s*=\s*(?!1525\b)\d+", text):
                errors.append("{} contains a non-1525 protocol in production config".format(entry.name))
            if re.search(r"httpLogin\s*=\s*true\b", text):
                errors.append("{} enables httpLogin in production config".format(entry.name))
            if re.search(r"useAuthenticator\s*=\s*true\b", text):
                errors.append("{} enables authenticator in production config".format(entry.name))

    folders: Dict[str, dict] = {}
    grouped: Dict[str, List[ArchiveEntry]] = defaultdict(list)
    for entry in entries:
        top = entry.name.split("/", 1)[0] if "/" in entry.name else "[root]"
        grouped[top].append(entry)
    for top, group in sorted(grouped.items()):
        folders[top] = {
            "files": len(group),
            "uncompressed_size": sum(entry.size for entry in group),
            "compressed_size": sum(entry.compressed_size for entry in group),
        }
    top100 = [
        {
            "path": entry.name,
            "uncompressed_size": entry.size,
            "compressed_size": entry.compressed_size,
            "sha256": _bytes_sha256(entry.data),
        }
        for entry in sorted(entries, key=lambda item: (-item.size, item.name.lower()))[:100]
    ]
    return AuditResult(
        archive=archive_name,
        files=len(entries),
        compressed_size=compressed_size,
        uncompressed_size=sum(entry.size for entry in entries),
        errors=sorted(set(errors)),
        warnings=sorted(set(warnings)),
        top100=top100,
        folders=folders,
    )


def audit_archive(
    path: Path,
    require_layout: bool = True,
    require_production_config: bool = True,
    allow_private_endpoints: bool = False,
) -> AuditResult:
    entries = read_archive(path)
    return audit_entries(
        entries,
        path.name,
        path.stat().st_size,
        require_layout,
        require_production_config,
        allow_private_endpoints,
    )


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    temporary = path.with_name(".{}.tmp".format(path.name))
    temporary.write_text(json.dumps(value, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    os.replace(str(temporary), str(path))


def _spdx_id(path: str) -> str:
    readable = re.sub(r"[^A-Za-z0-9.-]", "-", path)
    suffix = hashlib.sha256(path.encode("utf-8")).hexdigest()[:12]
    return "SPDXRef-File-{}-{}".format(readable, suffix)


def write_spdx(path: Path, records: Sequence[PackageFile], version: str, platform: str, namespace_hash: str, epoch: int) -> None:
    created = dt.datetime.fromtimestamp(epoch, tz=dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    files = []
    relationships = []
    verification_hashes = []
    for record in records:
        file_id = _spdx_id(record.archive_path)
        files.append(
            {
                "SPDXID": file_id,
                "fileName": "./" + record.archive_path,
                "checksums": [{"algorithm": "SHA256", "checksumValue": _file_sha256(record.source)}],
                "licenseConcluded": "NOASSERTION",
                "licenseInfoInFiles": ["NOASSERTION"],
                "copyrightText": "NOASSERTION",
            }
        )
        relationships.append({"spdxElementId": "SPDXRef-Package", "relationshipType": "CONTAINS", "relatedSpdxElement": file_id})
        verification_hashes.append(_file_sha1(record.source))
    verification_code = hashlib.sha1("".join(sorted(verification_hashes)).encode("ascii")).hexdigest()
    document = {
        "spdxVersion": "SPDX-2.3",
        "dataLicense": "CC0-1.0",
        "SPDXID": "SPDXRef-DOCUMENT",
        "name": "Thappy-{}-{}".format(version, platform),
        "documentNamespace": "https://thappy.cl/spdx/client/{}/{}/{}".format(version, platform, namespace_hash),
        "creationInfo": {"created": created, "creators": ["Tool: scripts/package-client.py"]},
        "documentDescribes": ["SPDXRef-Package"],
        "packages": [
            {
                "SPDXID": "SPDXRef-Package",
                "name": "Thappy Client ({})".format(platform),
                "versionInfo": version,
                "downloadLocation": "NOASSERTION",
                "filesAnalyzed": True,
                "packageVerificationCode": {"packageVerificationCodeValue": verification_code},
                "licenseConcluded": "NOASSERTION",
                "licenseDeclared": "NOASSERTION",
                "copyrightText": "NOASSERTION",
            }
        ],
        "files": files,
        "relationships": relationships,
    }
    write_json(path, document)


def _manifest_component(
    archive: Path,
    component: str,
    version: str,
    platform: str,
    base_url: str,
) -> dict:
    base_url = base_url.rstrip("/") + "/"
    return {
        "name": "{}-{}".format(component, platform),
        "version": version,
        "platforms": [platform],
        "url": urllib.parse.urljoin(base_url, urllib.parse.quote(archive.name)),
        "sha256": _file_sha256(archive),
        "size": archive.stat().st_size,
        "archive": "zip",
        "target": ".",
    }


def _lua_quote(value: str) -> str:
    return json.dumps(value, ensure_ascii=True)


def materialize_environment_records(
    temporary_root: Path,
    environment: str,
    channel: str,
    version: str,
    platform: str,
    epoch: int,
    build_commit: str,
    binary: Path,
    asset_version: str,
    login_url: Optional[str],
    login_port: Optional[int],
    protocol_version: Optional[int],
    http_login: Optional[bool],
    use_authenticator: Optional[bool],
    website_url: Optional[str],
    support_url: Optional[str],
    update_manifest_url: Optional[str],
    component_base_url: str,
) -> List[PackageFile]:
    """Create build-only environment files without modifying the source tree."""

    if environment not in {"production", "test", "local"}:
        raise ReleaseError("environment must be production, test, or local")
    if not asset_version or not re.fullmatch(r"[0-9A-Za-z._-]{1,64}", asset_version):
        raise ReleaseError("asset version must be a non-empty release identifier")
    expected_channel = {"production": "stable", "test": "test", "local": "local"}[environment]
    if channel != expected_channel:
        raise ReleaseError("{} environment requires {} channel".format(environment, expected_channel))
    if environment == "production":
        overrides = (login_url, login_port, protocol_version, http_login, use_authenticator, website_url, support_url, update_manifest_url)
        if any(value is not None for value in overrides):
            raise ReleaseError("production endpoint values are immutable and cannot be overridden")
        effective_url = CANONICAL_LOGIN_URL
        effective_port = 443
        effective_protocol = 1525
        effective_http_login = False
        effective_authenticator = False
        effective_website = "https://thappy.cl"
        effective_support = "https://thappy.cl"
        effective_manifest = "https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json"
    else:
        required = {
            "login_url": login_url,
            "login_port": login_port,
            "protocol_version": protocol_version,
            "http_login": http_login,
            "use_authenticator": use_authenticator,
            "website_url": website_url,
            "support_url": support_url,
            "update_manifest_url": update_manifest_url,
        }
        missing = sorted(name for name, value in required.items() if value is None)
        if missing:
            raise ReleaseError("test/local packages require explicit values for: {}".format(", ".join(missing)))
        for label, raw_url in (
            ("login", login_url),
            ("website", website_url),
            ("support", support_url),
            ("manifest", update_manifest_url),
        ):
            parsed = urllib.parse.urlparse(str(raw_url))
            if parsed.scheme not in {"http", "https"} or not parsed.hostname or parsed.username or parsed.password:
                raise ReleaseError("{} URL must be HTTP(S), include a host, and omit userinfo".format(label))
        if not (1 <= int(login_port) <= 65535):
            raise ReleaseError("login port must be between 1 and 65535")
        effective_url = str(login_url)
        effective_port = int(login_port)
        effective_protocol = int(protocol_version)
        if not (1 <= effective_protocol <= 65535):
            raise ReleaseError("protocol version must be between 1 and 65535")
        effective_http_login = bool(http_login)
        effective_authenticator = bool(use_authenticator)
        effective_website = str(website_url)
        effective_support = str(support_url)
        effective_manifest = str(update_manifest_url)

    generated = temporary_root / "generated"
    (generated / "config").mkdir(parents=True, exist_ok=True)
    build_date = dt.datetime.fromtimestamp(epoch, tz=dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    runtime_environment = "localDevelopment" if environment == "local" else environment
    build_config = """-- Generated by scripts/package-client.py. Do not edit.
return {
    environment = %s,
    channel = %s,
    clientVersion = %s,
    assetVersion = %s,
    buildCommit = %s,
    buildDate = %s
}
""" % (
        _lua_quote(runtime_environment),
        _lua_quote(channel),
        _lua_quote(version),
        _lua_quote(asset_version),
        _lua_quote(build_commit),
        _lua_quote(build_date),
    )
    (generated / "config" / "build_config.lua").write_text(build_config, encoding="utf-8")
    records = [PackageFile(generated / "config" / "build_config.lua", "config/build_config.lua", "core")]
    if environment != "production":
        def environment_lua(configured: bool, name: str) -> str:
            if not configured:
                return """        configured = false, locked = true, channel = %s, loginUrl = '', loginPort = 443,
        protocolVersion = 1525, httpLogin = false, useAuthenticator = false,
        websiteUrl = 'https://thappy.cl', supportUrl = 'https://thappy.cl', updateManifestUrl = '', servers = {}""" % _lua_quote("local" if name == "localDevelopment" else name)
            return """        configured = true, locked = true, channel = %s, loginUrl = %s, loginPort = %d,
        protocolVersion = %d, httpLogin = %s, useAuthenticator = %s,
        websiteUrl = %s, supportUrl = %s, updateManifestUrl = %s,
        servers = { [%s] = { port = %d, protocol = %d, httpLogin = %s, useAuthenticator = %s } }""" % (
                _lua_quote(channel), _lua_quote(effective_url), effective_port, effective_protocol,
                "true" if effective_http_login else "false", "true" if effective_authenticator else "false",
                _lua_quote(effective_website), _lua_quote(effective_support), _lua_quote(effective_manifest),
                _lua_quote(effective_url), effective_port, effective_protocol,
                "true" if effective_http_login else "false", "true" if effective_authenticator else "false",
            )

        selected_test = environment == "test"
        selected_local = environment == "local"
        environments_lua = """-- Generated non-production profile. Production constants remain immutable.
return {
    production = {
        configured = true, locked = true, channel = 'stable',
        loginUrl = 'https://login.thappy.cl/login.php', loginPort = 443, protocolVersion = 1525,
        httpLogin = false, useAuthenticator = false, websiteUrl = 'https://thappy.cl', supportUrl = 'https://thappy.cl',
        updateManifestUrl = 'https://github.com/ecantillano/otclient/releases/latest/download/manifest-stable.json',
        servers = { ['https://login.thappy.cl/login.php'] = { port = 443, protocol = 1525, httpLogin = false, useAuthenticator = false } }
    },
    test = {
%s
    },
    localDevelopment = {
%s
    }
}
""" % (environment_lua(selected_test, "test"), environment_lua(selected_local, "localDevelopment"))
        (generated / "config" / "environments.lua").write_text(environments_lua, encoding="utf-8")
        records.append(PackageFile(generated / "config" / "environments.lua", "config/environments.lua", "core"))
    marker = {
        "schema_version": SCHEMA_VERSION,
        "environment": environment,
        "channel": channel,
        "platform": platform,
        "version": version,
    }
    write_json(generated / "artifact-environment.json", marker)
    metadata = dict(marker)
    metadata.update(
        {
            "build_commit": build_commit,
            "build_date": build_date,
            "binary_sha256": _file_sha256(binary),
            "asset_version": asset_version,
            "login_url": effective_url,
            "login_port": effective_port,
            "protocol_version": effective_protocol,
            "http_login": effective_http_login,
            "use_authenticator": effective_authenticator,
        }
    )
    write_json(generated / "build-metadata.json", metadata)
    allowed_hosts = {
        "github.com",
        "objects.githubusercontent.com",
        "release-assets.githubusercontent.com",
        "login.thappy.cl",
    }
    for candidate_url in (
        effective_url,
        effective_manifest,
        effective_website,
        effective_support,
        component_base_url,
    ):
        hostname = urllib.parse.urlparse(candidate_url).hostname
        if hostname:
            allowed_hosts.add(hostname.lower().rstrip("."))
    launcher_config = {
        "schema_version": SCHEMA_VERSION,
        "default_channel": "test" if environment == "test" else "stable",
        "channels": {
            "stable": {"manifest_url": CANONICAL_STABLE_MANIFEST_URL},
            "test": {"manifest_url": effective_manifest if environment == "test" else ""},
        },
        "allowed_hosts": sorted(allowed_hosts),
        "client_executables": dict(PLATFORM_EXECUTABLES),
        "client_args": [],
        "delete_allowlist": list(DEFAULT_DELETE_PATHS),
        "preserve_paths": list(LAUNCHER_PRESERVE_PATHS),
    }
    write_json(generated / "launcher-config.json", launcher_config)
    records.extend([
        PackageFile(generated / "artifact-environment.json", "artifact-environment.json", "core"),
        PackageFile(generated / "build-metadata.json", "build-metadata.json", "core"),
        # Bootstrap is distributed for clean/manual installs but intentionally
        # omitted from OTA manifests, so the preserved local policy is never
        # overwritten by a remote update.
        PackageFile(generated / "launcher-config.json", "launcher-config.json", "bootstrap"),
    ])
    return records


def validate_manifest(manifest: Mapping[str, object]) -> None:
    if manifest.get("schema_version") != SCHEMA_VERSION:
        raise ReleaseError("manifest schema_version must be 1")
    channel = manifest.get("channel")
    if channel not in {"stable", "test"}:
        raise ReleaseError("manifest channel must be stable or test")
    version = manifest.get("version")
    if not isinstance(version, str):
        raise ReleaseError("manifest version is missing")
    validate_version(version)
    if manifest.get("protocol_version") != 1525:
        raise ReleaseError("manifest protocol_version must be 1525")
    asset_version = manifest.get("asset_version")
    if not isinstance(asset_version, str):
        raise ReleaseError("manifest asset_version is missing")
    if not asset_version or not re.fullmatch(r"[0-9A-Za-z._-]{1,64}", asset_version):
        raise ReleaseError("manifest asset_version is invalid")
    notes = urllib.parse.urlparse(str(manifest.get("release_notes_url", "")))
    if notes.scheme != "https" or not notes.hostname or notes.username or notes.password:
        raise ReleaseError("manifest release_notes_url must be HTTPS without userinfo")
    if not isinstance(manifest.get("mandatory"), bool):
        raise ReleaseError("manifest mandatory must be boolean")
    minimum_launcher = manifest.get("minimum_launcher_version", "")
    if not isinstance(minimum_launcher, str):
        raise ReleaseError("manifest minimum_launcher_version must be a string")
    if minimum_launcher:
        validate_version(minimum_launcher)
    expected_environment = "production" if channel == "stable" else "test"
    if manifest.get("environment") != expected_environment:
        raise ReleaseError("manifest environment must be {} for {} channel".format(expected_environment, channel))
    login_url = urllib.parse.urlparse(str(manifest.get("login_url", "")))
    if login_url.scheme != "https" or not login_url.hostname or login_url.username or login_url.password:
        raise ReleaseError("manifest login_url must be HTTPS without userinfo")
    login_port = manifest.get("login_port")
    if not isinstance(login_port, int) or isinstance(login_port, bool) or not (1 <= login_port <= 65535):
        raise ReleaseError("manifest login_port must be between 1 and 65535")
    if not isinstance(manifest.get("http_login"), bool):
        raise ReleaseError("manifest http_login must be boolean")
    if not isinstance(manifest.get("use_authenticator"), bool):
        raise ReleaseError("manifest use_authenticator must be boolean")
    if expected_environment == "production" and (
        manifest.get("login_url") != CANONICAL_LOGIN_URL
        or login_port != 443
        or manifest.get("http_login")
        or manifest.get("use_authenticator")
    ):
        raise ReleaseError("production manifest endpoint policy is not canonical")
    components = manifest.get("components")
    if not isinstance(components, list) or not components:
        raise ReleaseError("manifest has no components")
    names = set()
    for component in components:
        if not isinstance(component, dict):
            raise ReleaseError("manifest component must be an object")
        name = component.get("name")
        if not isinstance(name, str) or not name or "/" in name or "\\" in name:
            raise ReleaseError("invalid manifest component name")
        if name.lower() in names:
            raise ReleaseError("duplicate manifest component {}".format(name))
        names.add(name.lower())
        if component.get("archive") != "zip":
            raise ReleaseError("launcher components must use zip")
        if not isinstance(component.get("size"), int) or component["size"] <= 0:
            raise ReleaseError("component {} has invalid size".format(name))
        sha256 = component.get("sha256")
        if not isinstance(sha256, str) or not re.fullmatch(r"[0-9a-f]{64}", sha256):
            raise ReleaseError("component {} has invalid SHA-256".format(name))
        url = urllib.parse.urlparse(str(component.get("url", "")))
        if url.scheme != "https" or not url.hostname or url.username or url.password:
            raise ReleaseError("component {} URL must be HTTPS without userinfo".format(name))
        platforms = component.get("platforms", [])
        if any(platform not in {"windows", "linux"} for platform in platforms):
            raise ReleaseError("component {} has invalid platform".format(name))
    deletes = manifest.get("delete", [])
    if not isinstance(deletes, list):
        raise ReleaseError("manifest delete must be a list")
    for deleted in deletes:
        normalized = str(deleted).rstrip("/")
        safe_archive_path(normalized)
        if normalized.lower() in {"data", "modules", "mods", "bin"}:
            raise ReleaseError("manifest delete path is too broad: {}".format(deleted))


def merge_manifests(paths: Sequence[Path], output: Path) -> dict:
    if not paths:
        raise ReleaseError("at least one manifest fragment is required")
    manifests = [json.loads(path.read_text(encoding="utf-8")) for path in paths]
    for manifest in manifests:
        validate_manifest(manifest)
    identity_fields = (
        "channel",
        "version",
        "protocol_version",
        "asset_version",
        "release_notes_url",
        "mandatory",
        "minimum_launcher_version",
        "environment",
        "login_url",
        "login_port",
        "http_login",
        "use_authenticator",
    )
    identity = {tuple(json.dumps(manifest[field], sort_keys=True) for field in identity_fields) for manifest in manifests}
    if len(identity) != 1:
        raise ReleaseError("manifest fragments disagree on release policy")
    first = manifests[0]
    components = []
    deletes = set()
    published_values = set()
    for manifest in manifests:
        components.extend(manifest["components"])
        deletes.update(manifest.get("delete", []))
        if manifest.get("published_at"):
            published_values.add(manifest["published_at"])
    if len(published_values) > 1:
        raise ReleaseError("manifest fragments disagree on published_at")
    merged = {
        "schema_version": SCHEMA_VERSION,
        "channel": first["channel"],
        "version": first["version"],
        "protocol_version": first["protocol_version"],
        "asset_version": first["asset_version"],
        "release_notes_url": first["release_notes_url"],
        "mandatory": first["mandatory"],
        "minimum_launcher_version": first.get("minimum_launcher_version", ""),
        "environment": first["environment"],
        "login_url": first["login_url"],
        "login_port": first["login_port"],
        "http_login": first["http_login"],
        "use_authenticator": first["use_authenticator"],
        "published_at": next(iter(published_values), ""),
        "components": sorted(components, key=lambda item: item["name"]),
        "delete": sorted(deletes),
    }
    validate_manifest(merged)
    write_json(output, merged)
    return merged


def _baseline_compressed_size(path: Optional[Path]) -> Optional[int]:
    if path is None:
        return None
    value = json.loads(path.read_text(encoding="utf-8"))
    size = value.get("compressed_size")
    if not isinstance(size, int) or size <= 0:
        raise ReleaseError("baseline report has invalid compressed_size")
    return size


def enforce_size_gate(actual: int, budget: Optional[int], baseline: Optional[int]) -> dict:
    if budget is not None and actual > budget:
        raise ReleaseError("package size {} exceeds hard budget {}".format(actual, budget))
    regression_percent = None
    if baseline is not None:
        regression_percent = ((actual / baseline) - 1.0) * 100.0
        if actual > int(baseline * 1.10):
            raise ReleaseError("package size {} exceeds baseline {} by more than 10%".format(actual, baseline))
    return {
        "budget": budget,
        "measured_budget": int(actual * 1.10),
        "baseline": baseline,
        "regression_percent": regression_percent,
        "within_budget": budget is None or actual <= budget,
        "within_regression_gate": baseline is None or actual <= int(baseline * 1.10),
    }


def build_packages(
    source_root: Path,
    binary: Path,
    launcher_binary: Path,
    output_dir: Path,
    platform: str,
    version: str,
    channel: str,
    base_url: str,
    epoch: int,
    budget: Optional[int] = None,
    baseline_report: Optional[Path] = None,
    delete_paths: Sequence[str] = DEFAULT_DELETE_PATHS,
    environment: str = "production",
    runtime_dir: Optional[Path] = None,
    asset_version: str = "1525",
    release_notes_url: Optional[str] = None,
    mandatory: bool = False,
    login_url: Optional[str] = None,
    login_port: Optional[int] = None,
    protocol_version: Optional[int] = None,
    http_login: Optional[bool] = None,
    use_authenticator: Optional[bool] = None,
    website_url: Optional[str] = None,
    support_url: Optional[str] = None,
    update_manifest_url: Optional[str] = None,
    build_commit: str = "unknown",
) -> dict:
    validate_platform(platform)
    validate_version(version)
    validate_channel(channel)
    parsed_url = urllib.parse.urlparse(base_url)
    if parsed_url.scheme != "https" or not parsed_url.hostname:
        raise ReleaseError("base URL must use HTTPS")
    output_dir.mkdir(parents=True, exist_ok=True)
    records = collect_package_files(source_root, binary, launcher_binary, platform, runtime_dir)
    if environment != "production":
        records = [record for record in records if record.archive_path.lower() != "config/environments.lua"]
    generated_root = Path(tempfile.mkdtemp(prefix="thappy-build-profile-"))
    records.extend(
        materialize_environment_records(
            generated_root,
            environment,
            channel,
            version,
            platform,
            epoch,
            build_commit,
            binary,
            asset_version,
            login_url,
            login_port,
            protocol_version,
            http_login,
            use_authenticator,
            website_url,
            support_url,
            update_manifest_url,
            base_url,
        )
    )
    records = sorted(records, key=lambda record: record.archive_path.lower())

    label = PLATFORM_LABELS[platform]
    component_archives: Dict[str, Path] = {}
    component_audits: Dict[str, AuditResult] = {}
    for component in ("bootstrap", "core", "modules", "data", "assets"):
        component_records = [record for record in records if record.component == component]
        if not component_records:
            if component == "assets":
                continue
            raise ReleaseError("component {} is empty".format(component))
        archive = output_dir / "Thappy-{}-{}-{}.zip".format(label, version, component)
        write_zip(archive, component_records, epoch)
        component_audit = audit_archive(
            archive,
            require_layout=False,
            require_production_config=False,
            allow_private_endpoints=environment != "production",
        )
        if not component_audit.ok:
            raise ReleaseError("component {} failed audit: {}".format(component, "; ".join(component_audit.errors)))
        component_archives[component] = archive
        component_audits[component] = component_audit

    if platform == "windows":
        full_archive = output_dir / "Thappy-{}-{}.zip".format(label, version)
        write_zip(full_archive, records, epoch)
    else:
        full_archive = output_dir / "Thappy-{}-{}.tar.zst".format(label, version)
        write_tar_zst(full_archive, records, epoch)

    audit = audit_archive(
        full_archive,
        require_layout=True,
        require_production_config=environment == "production",
        allow_private_endpoints=environment != "production",
    )
    if not audit.ok:
        raise ReleaseError("full package failed audit: {}".format("; ".join(audit.errors)))

    baseline_components = {}
    if baseline_report is not None:
        baseline_payload = json.loads(baseline_report.read_text(encoding="utf-8"))
        raw_components = baseline_payload.get("components", {})
        if raw_components is not None and not isinstance(raw_components, dict):
            raise ReleaseError("baseline report components must be an object")
        for name, value in raw_components.items():
            compressed = value.get("compressed_size") if isinstance(value, dict) else None
            if not isinstance(compressed, int) or compressed <= 0:
                raise ReleaseError("baseline component {} has invalid compressed_size".format(name))
            baseline_components[name] = compressed

    size_gate = enforce_size_gate(
        full_archive.stat().st_size,
        budget,
        _baseline_compressed_size(baseline_report),
    )
    component_sizes = {}
    for name, archive in sorted(component_archives.items()):
        component_audit = component_audits[name]
        component_sizes[name] = {
            "archive": archive.name,
            "files": component_audit.files,
            "compressed_size": archive.stat().st_size,
            "uncompressed_size": component_audit.uncompressed_size,
            "size_gate": enforce_size_gate(archive.stat().st_size, None, baseline_components.get(name)),
        }
    size_report = audit.as_dict()
    size_report.update(
        {
            "platform": platform,
            "version": version,
            "package": full_archive.name,
            "compressed_size": full_archive.stat().st_size,
            "size_gate": size_gate,
            "components": component_sizes,
        }
    )
    report_path = output_dir / "size-report-{}.json".format(platform)
    write_json(report_path, size_report)

    published = dt.datetime.fromtimestamp(epoch, tz=dt.timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
    manifest_path: Optional[Path] = None
    if environment != "local":
        notes_url = release_notes_url or "https://github.com/ecantillano/otclient/releases/tag/client-v{}".format(version)
        manifest = {
            "schema_version": SCHEMA_VERSION,
            "channel": channel,
            "version": version,
            "protocol_version": 1525,
            "asset_version": asset_version,
            "release_notes_url": notes_url,
            "mandatory": bool(mandatory),
            "minimum_launcher_version": MINIMUM_LAUNCHER_VERSION,
            "environment": environment,
            "login_url": CANONICAL_LOGIN_URL if environment == "production" else str(login_url),
            "login_port": 443 if environment == "production" else int(login_port),
            "http_login": False if environment == "production" else bool(http_login),
            "use_authenticator": False if environment == "production" else bool(use_authenticator),
            "published_at": published,
            "components": [
                _manifest_component(component_archives[name], name, version, platform, base_url)
                # The launcher is distributed for manual replacement only. It
                # must never overwrite its own running executable.
                for name in ("core", "modules", "data", "assets")
                if name in component_archives
            ],
            "delete": sorted({path.replace("\\", "/") for path in delete_paths}),
        }
        validate_manifest(manifest)
        manifest_path = output_dir / "manifest-{}.json".format(platform)
        write_json(manifest_path, manifest)

    sbom_path = output_dir / "sbom-{}.spdx.json".format(platform)
    write_spdx(sbom_path, records, version, platform, _file_sha256(full_archive), epoch)

    checksum_targets = sorted(
        list(component_archives.values()) + [full_archive, report_path, sbom_path] + ([manifest_path] if manifest_path else []),
        key=lambda item: item.name,
    )
    checksums_path = output_dir / "SHA256SUMS-{}.txt".format(platform)
    # SHA256SUMS is a transport contract shared by Windows and Unix tools.
    # Writing bytes prevents Python's Windows text mode from turning LF into
    # CRLF, which makes the trailing CR part of each filename for `shasum -c`.
    checksums_path.write_bytes(
        "".join("{}  {}\n".format(_file_sha256(path), path.name) for path in checksum_targets).encode("utf-8")
    )
    shutil.rmtree(str(generated_root))
    return {
        "full_archive": str(full_archive),
        "components": {name: str(path) for name, path in component_archives.items()},
        "manifest": str(manifest_path) if manifest_path else None,
        "size_report": str(report_path),
        "sbom": str(sbom_path),
        "checksums": str(checksums_path),
    }


def extract_archive_safe(path: Path, destination: Path) -> List[str]:
    entries = read_archive(path)
    destination.mkdir(parents=True, exist_ok=True)
    extracted = []
    for entry in entries:
        relative = safe_archive_path(entry.name)
        target = (destination / Path(*PurePosixPath(relative).parts)).resolve()
        try:
            target.relative_to(destination.resolve())
        except ValueError:
            raise ReleaseError("archive entry escapes destination: {}".format(relative))
        target.parent.mkdir(parents=True, exist_ok=True)
        if target.exists():
            raise ReleaseError("archive entry would overwrite existing file: {}".format(relative))
        target.write_bytes(entry.data)
        target.chmod(0o755 if entry.mode & 0o111 else 0o644)
        extracted.append(relative)
    return extracted


def _is_preserved(path: str) -> bool:
    normalized = path.replace("\\", "/").lstrip("./")
    return any(normalized == prefix.rstrip("/") or normalized.startswith(prefix) for prefix in PRESERVE_PREFIXES)


def simulate_update(manifest_path: Path, package_dir: Path, platform: str, seed_files: Optional[Mapping[str, bytes]] = None) -> dict:
    validate_platform(platform)
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    validate_manifest(manifest)
    seed_files = dict(
        seed_files
        or {
            "config.otml": b"player settings",
            "logs/client.log": b"log",
            "modules/obsolete.txt": b"old",
            PLATFORM_LAUNCHERS[platform]: b"existing launcher",
        }
    )
    with tempfile.TemporaryDirectory(prefix="thappy-update-") as temporary:
        root = Path(temporary) / "install"
        root.mkdir()
        for relative, data in seed_files.items():
            safe = safe_archive_path(relative)
            target = root / Path(*PurePosixPath(safe).parts)
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes(data)
        preserved_before = {
            relative: _bytes_sha256(data) for relative, data in seed_files.items() if _is_preserved(relative)
        }

        for deleted in manifest.get("delete", []):
            relative = safe_archive_path(str(deleted).rstrip("/"))
            if _is_preserved(relative):
                raise ReleaseError("manifest attempts to delete preserved path {}".format(deleted))
            target = root / Path(*PurePosixPath(relative).parts)
            if target.is_dir():
                shutil.rmtree(str(target))
            elif target.exists():
                target.unlink()

        installed_components = []
        for component in manifest["components"]:
            if component.get("platforms") and platform not in component["platforms"]:
                continue
            filename = Path(urllib.parse.unquote(urllib.parse.urlparse(component["url"]).path)).name
            archive = package_dir / filename
            if not archive.is_file():
                raise ReleaseError("missing component archive {}".format(filename))
            if archive.stat().st_size != component["size"]:
                raise ReleaseError("component {} size mismatch".format(component["name"]))
            if _file_sha256(archive) != component["sha256"]:
                raise ReleaseError("component {} SHA-256 mismatch".format(component["name"]))
            component_entries = read_archive(archive)
            for entry in component_entries:
                relative = safe_archive_path(entry.name)
                target = root / Path(*PurePosixPath(relative).parts)
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(entry.data)
                target.chmod(0o755 if entry.mode & 0o111 else 0o644)
            installed_components.append(component["name"])

        for relative, expected in preserved_before.items():
            target = root / Path(*PurePosixPath(relative).parts)
            if not target.is_file() or _file_sha256(target) != expected:
                raise ReleaseError("update did not preserve {}".format(relative))
        executable = root / PLATFORM_EXECUTABLES[platform]
        if not executable.is_file():
            raise ReleaseError("updated install is missing {}".format(executable.name))
        launcher = root / PLATFORM_LAUNCHERS[platform]
        if not launcher.is_file():
            raise ReleaseError("updated install is missing {}".format(launcher.name))
        return {
            "ok": True,
            "platform": platform,
            "version": manifest["version"],
            "components": installed_components,
            "preserved": sorted(preserved_before),
        }


def verify_clean_install(package: Path, platform: str, environment: str = "production") -> dict:
    validate_platform(platform)
    if environment not in {"production", "test", "local"}:
        raise ReleaseError("environment must be production, test, or local")
    audit = audit_archive(
        package,
        require_layout=True,
        require_production_config=environment == "production",
        allow_private_endpoints=environment != "production",
    )
    if not audit.ok:
        raise ReleaseError("package audit failed: {}".format("; ".join(audit.errors)))
    with tempfile.TemporaryDirectory(prefix="thappy-clean-install-") as temporary:
        root = Path(temporary) / "client"
        extracted = extract_archive_safe(package, root)
        executable = root / PLATFORM_EXECUTABLES[platform]
        if not executable.is_file():
            raise ReleaseError("clean install is missing {}".format(executable.name))
        launcher = root / PLATFORM_LAUNCHERS[platform]
        if not launcher.is_file():
            raise ReleaseError("clean install is missing {}".format(launcher.name))
        for required in ("data", "modules"):
            if not (root / required).is_dir():
                raise ReleaseError("clean install is missing {}/".format(required))
        if platform == "linux" and not os.access(str(executable), os.X_OK):
            raise ReleaseError("Linux executable bit was not preserved")
        if platform == "linux" and not os.access(str(launcher), os.X_OK):
            raise ReleaseError("Linux launcher executable bit was not preserved")
        return {
            "ok": True,
            "platform": platform,
            "environment": environment,
            "files": len(extracted),
            "executable": executable.name,
        }
