#!/usr/bin/env bash
# Cross-compile the earmuff-lsp language server for the platforms the extension
# bundles, into editors/vscode/server/<os>-<arch>/. Run from anywhere; paths are
# resolved relative to this script.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ext_dir="$(cd "$here/.." && pwd)"
repo_root="$(cd "$ext_dir/../.." && pwd)"
out_root="$ext_dir/server"

# os/arch pairs -> VS Code-style platform folder name
targets=(
  "darwin arm64 darwin-arm64"
  "darwin amd64 darwin-x64"
  "linux amd64 linux-x64"
  "linux arm64 linux-arm64"
  "windows amd64 win32-x64"
)

go_bin="${GO:-go}"

rm -rf "$out_root"
for t in "${targets[@]}"; do
  read -r goos goarch folder <<<"$t"
  bin="earmuff-lsp"
  [ "$goos" = "windows" ] && bin="earmuff-lsp.exe"
  dest="$out_root/$folder"
  mkdir -p "$dest"
  echo "building $goos/$goarch -> server/$folder/$bin"
  (cd "$repo_root" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 \
    "$go_bin" build -trimpath -ldflags="-s -w" -o "$dest/$bin" ./cmd/earmuff-lsp)
done
echo "done: $(du -sh "$out_root" | cut -f1) in $out_root"
