#!/usr/bin/env python3
"""Helper script to generate a YouTube OAuth refresh token.

Usage:
    python3 get_refresh_token.py --client-secret ./client_secret.json \
        --scopes https://www.googleapis.com/auth/youtube.upload

The script launches a browser window for the Google account you want the
refresh token to belong to, then prints the refresh token and access token.
"""

from __future__ import annotations

import argparse
import json
import pathlib
from typing import List

from google_auth_oauthlib.flow import InstalledAppFlow


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--client-secret",
        required=True,
        help="Path to the OAuth client secret JSON downloaded from Google Cloud",
    )
    parser.add_argument(
        "--scopes",
        nargs="+",
        default=["https://www.googleapis.com/auth/youtube.upload"],
        help="OAuth scopes to request (space separated)",
    )
    parser.add_argument(
        "--no-local-server",
        action="store_true",
        help="Use console flow instead of local web server if you cannot open a port",
    )
    parser.add_argument(
        "--output",
        help="Optional path to write the resulting credentials as JSON",
    )
    parser.add_argument(
        "--env-file",
        help="Optional path to write shell-friendly env vars (YOUTUBE_*).",
    )
    parser.add_argument(
        "--quiet",
        action="store_true",
        help="Suppress human-readable instructions (useful for automation)",
    )
    return parser.parse_args()


def run_flow(
    client_secret_path: pathlib.Path,
    scopes: List[str],
    use_console: bool,
    output: pathlib.Path | None,
    env_file: pathlib.Path | None,
    quiet: bool,
) -> None:
    if not client_secret_path.exists():
        raise FileNotFoundError(f"Client secret file not found: {client_secret_path}")

    flow = InstalledAppFlow.from_client_secrets_file(str(client_secret_path), scopes=scopes)

    if use_console:
        creds = flow.run_console()
    else:
        creds = flow.run_local_server(port=0)

    payload = {
        "client_id": flow.client_config["client_id"],
        "client_secret": flow.client_config["client_secret"],
        "refresh_token": creds.refresh_token,
        "access_token": creds.token,
        "token_expiry": creds.expiry.isoformat() if creds.expiry else None,
    }

    if output:
        output.parent.mkdir(parents=True, exist_ok=True)
        with output.open("w", encoding="utf-8") as fh:
            json.dump(payload, fh, indent=2)
            fh.write("\n")

    if env_file:
        env_file.parent.mkdir(parents=True, exist_ok=True)
        lines = [
            f'YOUTUBE_CLIENT_ID="{payload["client_id"]}"',
            f'YOUTUBE_CLIENT_SECRET="{payload["client_secret"]}"',
            f'YOUTUBE_REFRESH_TOKEN="{payload["refresh_token"]}"',
        ]
        env_file.write_text("\n".join(lines) + "\n", encoding="utf-8")

    if not quiet:
        print("\nCopy these values into your environment (store securely):\n")
        print(json.dumps(payload, indent=2))
        print("\nExport the client_id, client_secret, and refresh_token as env vars before running the service.")


def main() -> None:
    args = parse_args()
    run_flow(
        pathlib.Path(args.client_secret),
        args.scopes,
        args.no_local_server,
        pathlib.Path(args.output) if args.output else None,
        pathlib.Path(args.env_file) if args.env_file else None,
        args.quiet,
    )


if __name__ == "__main__":
    main()
