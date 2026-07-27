#!/usr/bin/env python3
"""Generate local light/dark PNG star-history charts for a GitHub repository.

Run manually with a token that can read repository metadata:
    GITHUB_TOKEN=... python scripts/gen_star_history.py --refresh

The GitHub Actions workflow supplies the token through the STAR_HISTORY_TOKEN
repository secret and commits the generated images under assets/.
"""

from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timedelta, timezone
from pathlib import Path

import matplotlib

matplotlib.use("Agg")

import matplotlib.dates as mdates
import numpy as np
from matplotlib import pyplot as plt
from matplotlib.colors import LinearSegmentedColormap, to_rgba
from matplotlib.ticker import FuncFormatter

REPO = "Rescenix/ResceneAgent"
CACHE = Path(__file__).with_name(".star-history-cache.json")
ACCENT = "#f5a623"
THEMES = {
    "light": {"bg": "#ffffff", "text": "#1f2328", "subtext": "#6a737d", "grid": "#dfe3e8"},
    "dark": {"bg": "#0d1117", "text": "#e6edf3", "subtext": "#8b949e", "grid": "#272d35"},
}


def token() -> str | None:
    for name in ("GITHUB_TOKEN", "GH_TOKEN"):
        if value := os.getenv(name, "").strip():
            return value
    try:
        result = subprocess.run(["gh", "auth", "token"], capture_output=True, text=True, timeout=10)
        if result.returncode == 0 and result.stdout.strip():
            return result.stdout.strip()
    except (FileNotFoundError, subprocess.SubprocessError):
        pass
    return None


def request_json(url: str, headers: dict[str, str]) -> list[dict]:
    for attempt in range(4):
        try:
            with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=30) as response:
                payload = json.load(response)
                if not isinstance(payload, list):
                    raise RuntimeError(payload.get("message", "GitHub returned an unexpected response"))
                return payload
        except (urllib.error.URLError, urllib.error.HTTPError, RuntimeError) as error:
            if attempt == 3:
                raise RuntimeError(f"GitHub stargazer request failed: {error}") from error
            time.sleep(2**attempt)
    return []


def fetch_stars(repo: str, refresh: bool) -> list[str]:
    if CACHE.exists() and not refresh:
        return json.loads(CACHE.read_text(encoding="utf-8"))

    headers = {
        "Accept": "application/vnd.github.star+json",
        "X-GitHub-Api-Version": "2022-11-28",
        "User-Agent": "ResceneAgent-star-history",
    }
    if value := token():
        headers["Authorization"] = f"Bearer {value}"
    else:
        print("warning: no token found; GitHub requests are rate limited", file=sys.stderr)

    stars: list[str] = []
    page = 1
    while True:
        items = request_json(f"https://api.github.com/repos/{repo}/stargazers?per_page=100&page={page}", headers)
        if not items:
            break
        stars.extend(item["starred_at"] for item in items)
        page += 1
    stars.sort()
    CACHE.write_text(json.dumps(stars), encoding="utf-8")
    return stars


def series(stars: list[str]) -> tuple[np.ndarray, np.ndarray]:
    timestamps = [datetime.strptime(value, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc) for value in stars]
    now = datetime.now(timezone.utc)
    start = min(timestamps, default=now - timedelta(days=30))
    # Anchor both ends. This also makes an empty or one-star repository render
    # a real date range instead of a degenerate single-pixel chart.
    x = [mdates.date2num(start)] + [mdates.date2num(value) for value in timestamps] + [mdates.date2num(now)]
    y = [0] + list(range(1, len(timestamps) + 1)) + [len(timestamps)]
    return np.array(x), np.array(y)


def render(x: np.ndarray, y: np.ndarray, repo: str, theme: dict[str, str], output: Path) -> None:
    fig, axis = plt.subplots(figsize=(12, 6.2), dpi=160)
    fig.patch.set_facecolor(theme["bg"])
    axis.set_facecolor(theme["bg"])
    fig.subplots_adjust(left=0.075, right=0.97, top=0.86, bottom=0.10)
    span = max(1, x[-1] - x[0])
    axis.set_xlim(x[0], x[-1] + span * 0.03)
    axis.set_ylim(0, max(1, y[-1]) * 1.10)

    red, green, blue, _ = to_rgba(ACCENT)
    fade = LinearSegmentedColormap.from_list("fade", [(red, green, blue, 0.0), (red, green, blue, 0.35)])
    image = axis.imshow(np.linspace(0, 1, 256).reshape(-1, 1), aspect="auto", cmap=fade, origin="lower",
                        extent=[*axis.get_xlim(), 0, axis.get_ylim()[1]], zorder=1)
    fill_x = np.concatenate(([x[0]], x, [x[-1]]))
    fill_y = np.concatenate(([0.0], y, [0.0]))
    polygon = axis.fill(fill_x, fill_y, alpha=0, zorder=1)[0]
    image.set_clip_path(polygon)
    axis.plot(x, y, color=ACCENT, linewidth=7, alpha=0.10, solid_capstyle="round", zorder=2)
    axis.plot(x, y, color=ACCENT, linewidth=2.6, solid_capstyle="round", zorder=3)
    axis.scatter([x[-1]], [y[-1]], s=70, color=ACCENT, edgecolor=theme["bg"], linewidth=2.2, zorder=4)
    axis.annotate(f"{int(y[-1]):,} stars", (x[-1], y[-1]), xytext=(-6, 14), textcoords="offset points",
                  ha="right", fontsize=16, fontweight="bold", color=theme["text"])
    # README already has the “Star History” heading; keep the generated image
    # focused on the repository and chart instead of repeating that title.
    fig.text(0.075, 0.93, repo, fontsize=12.5, color=theme["subtext"])
    axis.yaxis.grid(True, color=theme["grid"], linewidth=0.9, linestyle=(0, (5, 4)))
    axis.set_axisbelow(True)
    for side in ("top", "right", "left"):
        axis.spines[side].set_visible(False)
    axis.spines["bottom"].set_color(theme["grid"])
    axis.tick_params(axis="both", length=0, labelsize=11.5, colors=theme["subtext"], pad=8)
    axis.xaxis.set_major_locator(mdates.AutoDateLocator())
    axis.xaxis.set_major_formatter(mdates.ConciseDateFormatter(axis.xaxis.get_major_locator()))
    axis.yaxis.set_major_formatter(FuncFormatter(lambda value, _position: f"{int(value):,}"))
    fig.savefig(output, facecolor=theme["bg"], bbox_inches="tight", pad_inches=0.3)
    plt.close(fig)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=REPO)
    parser.add_argument("--out-dir", type=Path, default=Path("assets"))
    parser.add_argument("--refresh", action="store_true")
    args = parser.parse_args()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    x, y = series(fetch_stars(args.repo, args.refresh))
    for name, theme in THEMES.items():
        output = args.out_dir / f"star-history-{name}.png"
        render(x, y, args.repo, theme, output)
        print(f"wrote {output}")


if __name__ == "__main__":
    main()
