"""Generate the GENT Chat icons: a yellow pixel bolt on a near-black tile.

Pure standard library (no Pillow). Run from the repository root:

    python -I build/make_icons.py

Writes build/appicon.png (512 px) and frontend/src/assets/trayicon.png (32 px).
`wails3 task build` turns appicon.png into the Windows .ico.
"""

import struct
import zlib

BOLT_ROWS = [
    "....####",
    "...####.",
    "..####..",
    ".#######",
    "#######.",
    "...###..",
    "..###...",
    "..##....",
    ".##.....",
    ".#......",
]

TILE = (0x10, 0x10, 0x10, 255)
BODY = (0xFF, 0xEB, 0x3B, 255)
LIT = (0xFF, 0xFB, 0xD1, 255)
CLEAR = (0, 0, 0, 0)


def render(size, tile=True):
    """Return rows of RGBA tuples for a size x size icon."""
    cols, rows = len(BOLT_ROWS[0]), len(BOLT_ROWS)
    margin = size * (0.19 if tile else 0.04)
    cell = (size - 2 * margin) / rows
    left = (size - cell * cols) / 2
    top = margin
    radius = size * 0.22

    def in_tile(x, y):
        cx = min(max(x, radius), size - radius)
        cy = min(max(y, radius), size - radius)

        return (x - cx) ** 2 + (y - cy) ** 2 <= radius**2

    out = []

    for py in range(size):
        row = []

        for px in range(size):
            x, y = px + 0.5, py + 0.5
            gx, gy = int((x - left) // cell), int((y - top) // cell)
            colour = TILE if tile and in_tile(x, y) else CLEAR

            if 0 <= gy < rows and 0 <= gx < cols and x >= left and y >= top and BOLT_ROWS[gy][gx] == "#":
                lit = gx == 0 or BOLT_ROWS[gy][gx - 1] != "#"
                colour = LIT if lit else BODY

            row.append(colour)

        out.append(row)

    return out


def write_png(path, pixels):
    size = len(pixels)
    raw = b"".join(b"\x00" + b"".join(bytes(p) for p in row) for row in pixels)

    def chunk(kind, data):
        body = kind + data

        return struct.pack(">I", len(data)) + body + struct.pack(">I", zlib.crc32(body) & 0xFFFFFFFF)

    with open(path, "wb") as f:
        f.write(b"\x89PNG\r\n\x1a\n")
        f.write(chunk(b"IHDR", struct.pack(">IIBBBBB", size, size, 8, 6, 0, 0, 0)))
        f.write(chunk(b"IDAT", zlib.compress(raw, 9)))
        f.write(chunk(b"IEND", b""))


if __name__ == "__main__":
    write_png("build/appicon.png", render(512))
    write_png("frontend/src/assets/trayicon.png", render(32, tile=False))
    print("wrote build/appicon.png and frontend/src/assets/trayicon.png")
