#!/usr/bin/env bash
#
# install-libvips.sh — build and install a recent libvips on Ubuntu from source.
#
# sharp-go requires libvips >= 8.16 (internal/vips/cgo.go). Ubuntu's apt ships
# 8.15, which lacks the WebP `smart-deblock` and GIF `keep-duplicate-frames`
# save options this codebase relies on — so we build the latest stable release
# (or a pinned version) from the upstream tarball.
#
# The build is C-ONLY (-Dcpp=false): sharp-go binds the libvips C API and never
# links vips-cpp, so the C++ binding, its libstdc++ dependency, and the extra
# build time are all skipped.
#
# Usage:
#   scripts/install-libvips.sh                       # latest stable
#   VIPS_VERSION=8.18.2 scripts/install-libvips.sh   # pin a version
#   PREFIX=/opt/vips    scripts/install-libvips.sh   # custom install prefix
#
set -euo pipefail

VIPS_VERSION="${VIPS_VERSION:-latest}"   # "latest" resolves the newest release
PREFIX="${PREFIX:-/usr/local}"           # install prefix
MIN_MAJOR=8
MIN_MINOR=16

# ── helpers ───────────────────────────────────────────────────────────────
c_blue=$'\033[34m'; c_green=$'\033[32m'; c_yellow=$'\033[33m'; c_red=$'\033[31m'; c_off=$'\033[0m'
log()  { printf '%s==>%s %s\n' "$c_blue"   "$c_off" "$*"; }
ok()   { printf '%s ok%s %s\n' "$c_green"  "$c_off" "$*"; }
warn() { printf '%swarn%s %s\n' "$c_yellow" "$c_off" "$*" >&2; }
die()  { printf '%serr%s %s\n'  "$c_red"    "$c_off" "$*" >&2; exit 1; }
have() { command -v "$1" >/dev/null 2>&1; }

[ "$(uname -s)" = Linux ] || die "this script targets Ubuntu/Linux only"
have apt-get || die "apt-get not found — this script targets Debian/Ubuntu"

SUDO=""
if [ "$(id -u)" -ne 0 ]; then
  have sudo || die "not root and sudo is unavailable; re-run as root"
  SUDO="sudo"
fi

# ── build dependencies (apt) ─────────────────────────────────────────────────
install_deps() {
  log "installing build dependencies via apt"
  $SUDO apt-get update -y
  # Required: build tooling + glib + the core codecs sharp-go uses.
  $SUDO apt-get install -y --no-install-recommends \
    build-essential meson ninja-build pkg-config ca-certificates curl xz-utils \
    libglib2.0-dev libexpat1-dev \
    libjpeg-dev libpng-dev libtiff-dev libwebp-dev libexif-dev \
    libheif-dev libimagequant-dev libcgif-dev liborc-0.4-dev liblcms2-dev libfftw3-dev
  # Optional codecs — install best-effort; a missing package must not abort.
  for p in libjxl-dev librsvg2-dev libpoppler-glib-dev libspng-dev libarchive-dev; do
    if $SUDO apt-get install -y --no-install-recommends "$p" 2>/dev/null; then
      ok "optional: $p"
    else
      warn "skipped optional dep (not in apt): $p"
    fi
  done
}

# ── resolve + download ───────────────────────────────────────────────────────
resolve_latest() {
  local api="https://api.github.com/repos/libvips/libvips/releases/latest"
  curl -fsSL "$api" | sed -n 's/.*"tag_name": *"v\{0,1\}\([0-9][0-9.]*\)".*/\1/p' | head -n1
}

# ── build ────────────────────────────────────────────────────────────────────
build_source() {
  local ver="$VIPS_VERSION"
  [ "$ver" = latest ] && ver="$(resolve_latest)"
  [ -n "$ver" ] || die "could not resolve the latest libvips version"
  log "building libvips $ver from source (C-only, prefix=$PREFIX)"

  local tmp; tmp="$(mktemp -d)"
  trap '[ -n "${tmp:-}" ] && rm -rf "$tmp"' EXIT

  local tarball="$tmp/vips-$ver.tar.xz"
  curl -fSL "https://github.com/libvips/libvips/releases/download/v$ver/vips-$ver.tar.xz" -o "$tarball"
  tar -xJf "$tarball" -C "$tmp"

  ( cd "$tmp/vips-$ver"
    # -Dcpp=false: build the C library only — sharp-go never links vips-cpp.
    meson setup build \
      --prefix="$PREFIX" \
      --buildtype=release \
      -Dcpp=false \
      -Ddeprecated=false \
      -Dintrospection=disabled \
      -Dexamples=false
    meson compile -C build
    $SUDO meson install -C build
  )

  # Refresh the dynamic linker cache so the new .so is resolvable.
  if have ldconfig; then $SUDO ldconfig "$PREFIX/lib" 2>/dev/null || $SUDO ldconfig 2>/dev/null || true; fi
  ok "libvips $ver installed to $PREFIX"
}

# ── verify ───────────────────────────────────────────────────────────────────
version_ok() { # version_ok <major> <minor>
  [ "$1" -gt "$MIN_MAJOR" ] || { [ "$1" -eq "$MIN_MAJOR" ] && [ "$2" -ge "$MIN_MINOR" ]; }
}

verify() {
  log "verifying libvips"
  # A custom PREFIX may not be on the default pkg-config search path yet.
  export PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig${PKG_CONFIG_PATH:+:$PKG_CONFIG_PATH}"
  if have pkg-config && pkg-config --exists vips 2>/dev/null; then
    local v maj min; v="$(pkg-config --modversion vips)"
    maj="${v%%.*}"; min="${v#*.}"; min="${min%%.*}"
    if version_ok "$maj" "$min"; then
      ok "pkg-config sees vips $v (>= $MIN_MAJOR.$MIN_MINOR) — sharp-go's gate will pass"
    else
      warn "pkg-config sees vips $v, but sharp-go needs >= $MIN_MAJOR.$MIN_MINOR"
    fi
  else
    warn "pkg-config can't find vips on the default path"
  fi

  cat <<EOF

If $PREFIX is not on the default search path, export these (add to your profile
or CI env) before building sharp-go:

  export PKG_CONFIG_PATH="$PREFIX/lib/pkgconfig\${PKG_CONFIG_PATH:+:\$PKG_CONFIG_PATH}"
  export LD_LIBRARY_PATH="$PREFIX/lib\${LD_LIBRARY_PATH:+:\$LD_LIBRARY_PATH}"

Then confirm sharp-go links it:
  ${c_green}go run ./cmd/sharpgo-doctor${c_off}   (or: make doctor)
EOF
}

# ── main ──────────────────────────────────────────────────────────────────────
log "target: libvips $VIPS_VERSION  (Ubuntu, source build, prefix=$PREFIX)"
install_deps
build_source
verify
