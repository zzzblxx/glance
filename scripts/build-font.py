"""Generate a deterministic, fixed-address BMP bitmap font from upstream Unifont.

This generates a binary asset, not application source. Each Unicode BMP glyph
uses 33 bytes: width (8 or 16), then sixteen big-endian 16-bit bitmap rows.
The original glyphs are unchanged; narrow rows are padded on the right.
"""
import gzip
from pathlib import Path
import hashlib

root = Path(__file__).resolve().parent.parent
source = root / "assets/fonts/unifont-16.0.03.hex.gz"
if hashlib.sha256(source.read_bytes()).hexdigest() != "4f4afe8f475a2639ece3cdf96b6f128663f18f2b859e6685dff83d5b72a07b23":
    raise SystemExit("Unexpected upstream font checksum")
glyphs = bytearray(65536 * 33)
for line in gzip.decompress(source.read_bytes()).decode("ascii").splitlines():
    code, bitmap = line.split(":")
    code = int(code, 16)
    if code > 0xFFFF or len(bitmap) not in (32, 64):
        continue
    width = 8 if len(bitmap) == 32 else 16
    raw = bytes.fromhex(bitmap)
    offset = code * 33
    glyphs[offset] = width
    for row in range(16):
        bits = raw[row:row+1] + b"\x00" if width == 8 else raw[row*2:row*2+2]
        glyphs[offset+1+row*2:offset+3+row*2] = bits
out = root / "assets/fonts/unifont16.bin"
out.write_bytes(glyphs)
print(f"Built {out.name}: {len(glyphs)} bytes, SHA256 {hashlib.sha256(glyphs).hexdigest()}")
