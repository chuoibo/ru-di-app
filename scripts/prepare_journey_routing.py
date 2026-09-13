#!/usr/bin/env python3
"""Prepare pinned public OSM data and a private Valhalla config outside Git."""

from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import urllib.request

IMAGE = "ghcr.io/valhalla/valhalla-scripted:3.8.3@sha256:24ef7955899dececb94e26c6dfb89d64fabfae875f980432694b0261eb6c251b"
DEFAULT_URL = "https://download.geofabrik.de/asia/vietnam-260911.osm.pbf"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--data-dir", required=True, type=Path)
    parser.add_argument("--pbf-url", default=DEFAULT_URL)
    args = parser.parse_args()
    directory = args.data_dir.expanduser().resolve()
    repo = Path(__file__).resolve().parents[1]
    if directory == repo or repo in directory.parents:
        parser.error("Dữ liệu routing phải ở ngoài repository.")
    directory.mkdir(parents=True, exist_ok=True)
    pbf = directory / Path(args.pbf_url).name
    if not pbf.name.endswith(".osm.pbf") or not args.pbf_url.startswith(
        "https://download.geofabrik.de/"
    ):
        parser.error("Chỉ nhận bản trích OSM công khai từ Geofabrik qua HTTPS.")
    if not pbf.exists():
        temporary = pbf.with_suffix(".download")
        with (
            urllib.request.urlopen(args.pbf_url, timeout=60) as source,
            temporary.open("wb") as target,
        ):
            while chunk := source.read(1024 * 1024):
                target.write(chunk)
        temporary.replace(pbf)
    digest = hashlib.file_digest(pbf.open("rb"), "sha256").hexdigest()
    graph_version = f"vietnam-3.8.3-{digest[:16]}"
    manifest_path = directory / "graph-manifest.json"
    manifest = {
        "image": IMAGE,
        "pbf_url": args.pbf_url,
        "pbf_sha256": digest,
        "graph_version": graph_version,
        "traffic": "none",
        "attribution": "© OpenStreetMap contributors, ODbL 1.0",
    }
    if manifest_path.exists() and json.loads(manifest_path.read_text()) != manifest:
        parser.error(
            "Thư mục đã có phiên bản khác; dùng thư mục mới để giữ khả năng rollback."
        )
    raw = subprocess.check_output(
        ["docker", "run", "--rm", "--entrypoint", "valhalla_build_config", IMAGE],
        text=True,
    )
    config = json.loads(raw)
    config["logging"] = {"type": "file", "file_name": "/dev/null", "color": False}
    # Fifty actual stops plus a duplicated start for a return trip.
    for mode in ("auto", "motor_scooter", "pedestrian"):
        config["service_limits"][mode]["max_locations"] = 51
        config["service_limits"][mode]["max_matrix_location_pairs"] = 2500
    config["mjolnir"]["concurrency"] = 2
    config["mjolnir"]["max_cache_size"] = 256_000_000
    config["mjolnir"]["id_table_size"] = 100_000_000
    (directory / "valhalla.json").write_text(json.dumps(config, indent=2) + "\n")
    manifest_path.write_text(json.dumps(manifest, indent=2) + "\n")
    print(
        json.dumps(
            {
                "MOBILE_ROUTING_DATA_DIR": str(directory),
                "MOBILE_ROUTING_GRAPH_VERSION": graph_version,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
