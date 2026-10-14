# Quota banners

Four original 704 × 176 JPEGs supplied by the project owner; preserved
without scaling, cropping or re-illustration.

## License

Copyright (c) 2026 zzzblxx.

The project owner licenses all four JPEGs and their generated embedded data
in `firmware/shared/banner_assets.h` under the [MIT License](../../LICENSE).
Use, modification, redistribution and commercial use are permitted subject to
retaining the copyright and permission notice. These images are not covered
by the Unifont or Go font licenses.

| File / resource ID | Effective remaining quota | Caption |
| --- | --- | --- |
| sprint.jpg / sprint | ≥ 70% | 站起来蹬！ |
| cruise.jpg / cruise | ≥ 30%, < 70% | 定速巡航 |
| snack.jpg / snack | ≥ 1%, < 30% | 要吃辣堡 |
| sleep.jpg / sleep | < 1% | 玛卡巴卡 |

Use the bucket's five-hour window when present, otherwise its confirmed weekly-only
window. Add 100 percentage points per known available reset. Unknown reset counts
contribute nothing; unavailable/ambiguous quota shows a neutral waiting message.
Stale cached quota retains its selected image and the existing stale warning.
Only image selection uses this score; actual quota bars and reset counts do not change.

`go run ./cmd/build-banners` converts these originals to nearest 16-level grayscale
and writes `firmware/shared/banner_assets.h`. Each byte stores a 1–16 pixel run
in its upper nibble (length minus one) and Gray4 value in its lower nibble.
Original JPEG SHA256 values are recorded in the generated header. Firmware and
Mac preview compile the same embedded bytes and decode into their existing frame.
USB carries only the resource ID, never image bytes.
