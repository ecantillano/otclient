#!/usr/bin/env python3
"""Build deterministic Thappy release and launcher component packages."""

import argparse
import json
import os
import subprocess
import sys
from pathlib import Path

from client_release import ReleaseError, build_packages, merge_manifests


def parse_bool(value: str) -> bool:
    lowered = value.lower()
    if lowered == "true":
        return True
    if lowered == "false":
        return False
    raise argparse.ArgumentTypeError("expected true or false")


def parser() -> argparse.ArgumentParser:
    root = argparse.ArgumentParser(description=__doc__)
    commands = root.add_subparsers(dest="command", required=True)

    package = commands.add_parser("package", help="package one platform")
    package.add_argument("--source-root", type=Path, default=Path.cwd())
    package.add_argument("--binary", type=Path, required=True)
    package.add_argument("--launcher-binary", type=Path, required=True)
    package.add_argument("--runtime-dir", type=Path)
    package.add_argument("--output-dir", type=Path, required=True)
    package.add_argument("--platform", choices=("windows", "linux"), required=True)
    package.add_argument("--version", required=True)
    package.add_argument("--asset-version", default="1525")
    package.add_argument("--channel", choices=("stable", "test", "local"), required=True)
    package.add_argument("--environment", choices=("production", "test", "local"), required=True)
    package.add_argument("--base-url", required=True)
    package.add_argument("--release-notes-url")
    package.add_argument("--mandatory", action="store_true")
    package.add_argument("--source-date-epoch", type=int, default=int(os.environ.get("SOURCE_DATE_EPOCH", "0")))
    package.add_argument("--build-commit", default=os.environ.get("GITHUB_SHA", "unknown"))
    package.add_argument("--budget", type=int)
    package.add_argument("--baseline-report", type=Path)
    package.add_argument("--delete", action="append", default=[])

    package.add_argument("--login-url")
    package.add_argument("--login-port", type=int)
    package.add_argument("--protocol-version", type=int)
    package.add_argument("--http-login", type=parse_bool)
    package.add_argument("--use-authenticator", type=parse_bool)
    package.add_argument("--website-url")
    package.add_argument("--support-url")
    package.add_argument("--update-manifest-url")

    merge = commands.add_parser("merge-manifests", help="merge platform manifest fragments")
    merge.add_argument("--output", type=Path, required=True)
    merge.add_argument("manifests", nargs="+", type=Path)
    return root


def main() -> int:
    args = parser().parse_args()
    try:
        if args.command == "merge-manifests":
            result = merge_manifests(args.manifests, args.output)
        else:
            kwargs = {}
            if args.delete:
                kwargs["delete_paths"] = args.delete
            result = build_packages(
                source_root=args.source_root,
                binary=args.binary,
                launcher_binary=args.launcher_binary,
                runtime_dir=args.runtime_dir,
                output_dir=args.output_dir,
                platform=args.platform,
                version=args.version,
                asset_version=args.asset_version,
                channel=args.channel,
                environment=args.environment,
                base_url=args.base_url,
                release_notes_url=args.release_notes_url,
                mandatory=args.mandatory,
                epoch=args.source_date_epoch,
                build_commit=args.build_commit,
                budget=args.budget,
                baseline_report=args.baseline_report,
                login_url=args.login_url,
                login_port=args.login_port,
                protocol_version=args.protocol_version,
                http_login=args.http_login,
                use_authenticator=args.use_authenticator,
                website_url=args.website_url,
                support_url=args.support_url,
                update_manifest_url=args.update_manifest_url,
                **kwargs
            )
    except (OSError, ReleaseError, subprocess.CalledProcessError) as error:
        print("package-client: {}".format(error), file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    sys.exit(main())
