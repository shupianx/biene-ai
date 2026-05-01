"""Detect each robot's "screen" rect inside renderer/src/assets/avatar.png.

The sprite is a 250×200 sheet laid out 5 cols × 4 rows of 50×50 cells. We
infer the screen rectangle in each cell by:

  1. Building a "dark" mask (RGB sum < threshold) — covers both the body
     outline and the screen interior.
  2. Eroding by 1 px so the 1-px-thick outline disappears but the filled
     screen survives.
  3. Running a flood-fill connected-components pass and picking the
     largest blob whose centre lies inside the cell's central band
     (avoids picking up dark feet / antennas).
  4. Returning that blob's bounding box, expanded by 1 px to recapture
     the screen border that was lost during erosion.

Pure-Python (PIL only — no numpy/scipy) so it runs on the project's
default toolchain.
"""

from __future__ import annotations

import sys
from collections import deque
from pathlib import Path

from PIL import Image, ImageDraw

SPRITE_PATH = Path(__file__).resolve().parent.parent / "renderer" / "src" / "assets" / "avatar.png"
COLS, ROWS, CELL = 5, 4, 50
TOTAL = COLS * ROWS

# Pixels darker than this on summed RGB are considered "dark". 90 keeps the
# screens (often pure black) and outlines (~0–60) while excluding the
# light/medium body shells of every bot in the sheet.
DARK_THRESHOLD = 90

# Screens always sit in the upper-middle of the face; restricting blobs to
# a vertical band keeps stray dark accents (mouths, feet) from winning the
# largest-component vote.
CENTRAL_Y_MIN = 8
CENTRAL_Y_MAX = 42


def is_dark(pixel: tuple[int, int, int, int]) -> bool:
    r, g, b, a = pixel
    if a < 128:
        return False
    return (r + g + b) < DARK_THRESHOLD


def erode(mask: list[list[bool]]) -> list[list[bool]]:
    """3×3 cross erosion: a pixel survives only if itself + 4 neighbours
    are all True. Drops 1-px outlines while keeping filled rectangles."""
    h = len(mask)
    w = len(mask[0]) if h else 0
    out = [[False] * w for _ in range(h)]
    for y in range(h):
        for x in range(w):
            if not mask[y][x]:
                continue
            if y == 0 or y == h - 1 or x == 0 or x == w - 1:
                continue
            if (
                mask[y - 1][x]
                and mask[y + 1][x]
                and mask[y][x - 1]
                and mask[y][x + 1]
            ):
                out[y][x] = True
    return out


def connected_components(mask: list[list[bool]]) -> list[list[tuple[int, int]]]:
    h = len(mask)
    w = len(mask[0]) if h else 0
    seen = [[False] * w for _ in range(h)]
    blobs: list[list[tuple[int, int]]] = []
    for y in range(h):
        for x in range(w):
            if not mask[y][x] or seen[y][x]:
                continue
            blob: list[tuple[int, int]] = []
            q: deque[tuple[int, int]] = deque([(x, y)])
            seen[y][x] = True
            while q:
                cx, cy = q.popleft()
                blob.append((cx, cy))
                for dx, dy in ((-1, 0), (1, 0), (0, -1), (0, 1)):
                    nx, ny = cx + dx, cy + dy
                    if 0 <= nx < w and 0 <= ny < h and mask[ny][nx] and not seen[ny][nx]:
                        seen[ny][nx] = True
                        q.append((nx, ny))
            blobs.append(blob)
    return blobs


def best_screen_rect(cell_pixels) -> tuple[int, int, int, int] | None:
    mask = [[is_dark(cell_pixels[y * CELL + x]) for x in range(CELL)] for y in range(CELL)]
    eroded = erode(mask)
    blobs = connected_components(eroded)
    if not blobs:
        return None

    def score(blob: list[tuple[int, int]]) -> tuple[int, int]:
        cy = sum(p[1] for p in blob) / len(blob)
        in_band = CENTRAL_Y_MIN <= cy <= CENTRAL_Y_MAX
        # Prefer blobs in the vertical band; fall back to size if no
        # candidate sits in the band.
        return (1 if in_band else 0, len(blob))

    blob = max(blobs, key=score)
    xs = [p[0] for p in blob]
    ys = [p[1] for p in blob]
    x0, x1 = min(xs), max(xs)
    y0, y1 = min(ys), max(ys)
    # Re-expand by 1 px to recapture the border erosion stripped off, then
    # clamp to the cell bounds.
    x0 = max(0, x0 - 1)
    y0 = max(0, y0 - 1)
    x1 = min(CELL - 1, x1 + 1)
    y1 = min(CELL - 1, y1 + 1)
    return (x0, y0, x1 - x0 + 1, y1 - y0 + 1)


def main() -> None:
    img = Image.open(SPRITE_PATH).convert("RGBA")
    px = list(img.getdata())
    width = img.width

    rects: list[tuple[int, int, int, int] | None] = []
    for index in range(TOTAL):
        col = index % COLS
        row = index // COLS
        cell = []
        for cy in range(CELL):
            base = (row * CELL + cy) * width + col * CELL
            cell.extend(px[base : base + CELL])
        rects.append(best_screen_rect(cell))

    if "--debug" in sys.argv:
        write_debug(img, rects)
        return

    # Emit JSON suitable for direct paste into TypeScript.
    print("[")
    for i, rect in enumerate(rects):
        if rect is None:
            print(f"  /* {i:>2} */ null,")
            continue
        x, y, w, h = rect
        print(f"  /* {i:>2} */ {{ x: {x}, y: {y}, w: {w}, h: {h} }},")
    print("]")


def write_debug(img: Image.Image, rects) -> None:
    # Scale up 6× so individual pixels and the 1-px overlay rect are easy
    # to inspect visually, then paint each detected rect in red.
    scale = 6
    debug = img.resize(
        (img.width * scale, img.height * scale),
        resample=Image.Resampling.NEAREST,
    ).convert("RGBA")
    draw = ImageDraw.Draw(debug)
    for index, rect in enumerate(rects):
        if rect is None:
            continue
        col = index % COLS
        row = index // COLS
        x, y, w, h = rect
        x0 = (col * CELL + x) * scale
        y0 = (row * CELL + y) * scale
        x1 = x0 + w * scale - 1
        y1 = y0 + h * scale - 1
        draw.rectangle((x0, y0, x1, y1), outline=(255, 0, 80, 255), width=2)
    out_path = Path(__file__).resolve().parent / "avatar_screens_debug.png"
    debug.save(out_path)
    print(f"wrote {out_path}")


if __name__ == "__main__":
    main()
