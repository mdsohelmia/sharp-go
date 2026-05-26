# proxy — image-optimising HTTP proxy (Fastly Image Optimizer drop-in)

Same shape as `image-optimizer-edge-proxy/bunproxy`, rewritten on
`sharp-go`. Adds AVIF + Accept-header format negotiation to beat
Fastly Image Optimizer on size at equivalent (or better) perceptual
quality.

## Run

```bash
go run ./examples/proxy
# PORT=3003 (default), UPSTREAM_BASE=https://staging.aarong.com (default)
```

## Routes

| Path                         | Behaviour                                      |
| ---------------------------- | ---------------------------------------------- |
| `/`                          | usage banner                                   |
| `/health`                    | liveness probe                                 |
| `/<upstream-path>?…`         | fetch upstream image, encode AVIF/WebP/JPEG, stream back |
| `/raw/<upstream-path>`       | pass-through (no transcode)                    |
| `/fastly/<path>?…`           | streams the same path from `FASTLY_BASE` so the compare HTML can fetch it same-origin |
| `/metrics[/<path>]`          | butteraugli + ssimulacra2 (JSON)               |
| `/compare[/<path>]`          | side-by-side HTML viewer: original \| fastly \| sharp-go |

## Environment

| Var            | Default                          | Purpose                                                  |
| -------------- | -------------------------------- | -------------------------------------------------------- |
| `PORT`         | `3003`                           | listen port                                              |
| `UPSTREAM_BASE`| `https://staging.aarong.com`     | image origin                                             |
| `FASTLY_BASE`  | `https://mcprod.aarong.com`      | Fastly Image Optimizer endpoint shown in the middle pane |
| `DEFAULT_PATH` | `/media/catalog/product/0/5/0560000084696.jpg` | path used by `/compare` and `/metrics` when none given |

Set `FASTLY_BASE=` (explicitly empty) to disable the 3rd pane and fall back to a 2-pane original-vs-sharp-go view.

## Query params

| Param      | Values                                              | Default     |
| ---------- | --------------------------------------------------- | ----------- |
| `w`        | `1..8000`                                           | none        |
| `h`        | `1..8000`                                           | none        |
| `q`        | `1..100`                                            | preset      |
| `fit`      | `cover` / `contain` / `fill` / `outside` / `bounds` | `bounds`    |
| `effort` / `optimize` | `low` / `medium` / `high` / `max`        | `medium`    |
| `format` / `f` | `auto` / `avif` / `webp` / `jpeg`               | `auto`      |
| `bg-color` | `r,g,b` or `r,g,b,a` (each 0–255)                   | none        |

`format=auto` negotiates via the `Accept` request header:
**AVIF > WebP > JPEG**. Modern Chrome/Safari/Firefox advertise AVIF
and get the smallest payload.

## Response headers

| Header              | Meaning                                  |
| ------------------- | ---------------------------------------- |
| `Content-Type`      | actual encoder used                      |
| `Vary: Accept`      | tells CDNs to cache per-Accept           |
| `X-Image-Format`    | `avif` / `webp` / `jpeg`                 |
| `X-Image-Quality`   | resolved quality (after preset + `q=`)   |
| `X-Image-Effort`    | resolved encoder effort                  |
| `X-Upstream`        | full upstream URL                        |

## Per-format preset tuning

Calibrated against ssimulacra2 + butteraugli on typical product photos.
AVIF values look low but AV1 q=50 ≈ WebP q=80 perceptually.

|         | low       | medium    | high      | max       |
| ------- | --------- | --------- | --------- | --------- |
| AVIF    | q45 e3    | q50 e4    | q50 e6    | q55 e9    |
| WebP    | q70 e2    | q75 e4    | q75 e6    | q80 e6    |
| JPEG    | q70       | q78 moz   | q80 moz   | q85 moz   |

## Benchmark — `aarong.com/0560000084696.jpg` at 1080×1440

Source: Fastly Image Optimizer (mcprod) vs this proxy. Reference is the
original 1800×2400 JPEG downscaled to 1080×1440 with `vips thumbnail`.
Same `Accept: image/avif,image/webp,*/*`. Lower butteraugli = better;
higher ssimulacra2 = better.

| Pipeline                  | Bytes   | Butteraugli ↓ | SSIMULACRA2 ↑ |
| ------------------------- | ------- | ------------- | ------------- |
| Fastly Image Optimizer    | 129,930 | 1.53          | 65.21         |
| sharp-go WebP (high q=75) | 144,966 | 1.44          | 67.94         |
| sharp-go AVIF (high q=50) |  99,910 | 1.59          | 65.48         |
| **sharp-go AVIF (max)**   | 123,844 | **1.33**      | **70.84**     |

**AVIF `max` is 5% smaller than Fastly AND has the best quality
scores in the test.** AVIF `high` is 23% smaller than Fastly at
equivalent quality.

## Example

```bash
# Auto-negotiate format from Accept (recommended for end users)
curl -H 'Accept: image/avif,image/webp,*/*' \
  -o out.bin 'http://localhost:3003/path/to/img.jpg?w=1080&effort=high'

# Force AVIF, max effort
curl -o out.avif \
  'http://localhost:3003/path/to/img.jpg?w=1080&format=avif&effort=max'

# Interactive A/B page
open 'http://localhost:3003/compare?w=1080&effort=high'
```

`/metrics` requires `butteraugli_main` and `ssimulacra2` on `$PATH`.
