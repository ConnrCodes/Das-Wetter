#!/bin/sh
set -eu

# Resolve the repository independently of the caller's current directory.
ROOT=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
PREFIX=${PREFIX:-"$HOME/.local/bin"}
TARGET="$PREFIX/weather"
TMP="$PREFIX/.weather-install.$$"

cleanup() {
  rm -f "$TMP"
}
trap cleanup EXIT HUP INT TERM

mkdir -p "$PREFIX"

if command -v go >/dev/null 2>&1; then
  (cd "$ROOT" && CGO_ENABLED=0 go build -trimpath -ldflags='-s -w -buildid=' -o "$TMP" .)
elif [ -x "$ROOT/weather" ]; then
  cp "$ROOT/weather" "$TMP"
else
  echo "weather: Go is not installed and no prebuilt binary was found at $ROOT/weather" >&2
  exit 1
fi

chmod 0755 "$TMP"
mv -f "$TMP" "$TARGET"

# Convenience names. The `das` alias accepts `das wetter` via main.go.
ln -sf "$TARGET" "$PREFIX/das-wetter"
ln -sf "$TARGET" "$PREFIX/das"

echo "Installed Das wetter to $TARGET"
echo "Run it with: weather"
case ":$PATH:" in
  *":$PREFIX:"*) ;;
  *) echo "Add to ~/.zshrc: export PATH=\"$PREFIX:\$PATH\"" ;;
esac
