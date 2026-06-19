#!/usr/bin/env bash
# Copy the repo's example songs into the playground's static dir and write a
# manifest.json the front end uses to populate the "example" picker. The title
# is taken from each file's first `//` comment line, falling back to the name.
#
# Run from the repo root:  bash website/scripts/playground-examples.sh
set -euo pipefail

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
src_dir="$repo_root/examples"
dest_dir="$repo_root/website/static/playground/examples"

mkdir -p "$dest_dir"
rm -f "$dest_dir"/*.ear "$dest_dir/manifest.json"

manifest="["
first=1
for f in "$src_dir"/*.ear; do
  [ -e "$f" ] || continue
  name="$(basename "$f")"
  # Skip scratch files.
  case "$name" in example.ear | exp.ear) continue ;; esac
  cp "$f" "$dest_dir/$name"
  # Title: first // comment, stripped of leading slashes and any trailing
  # "earmuff v2" boilerplate (with its separator). Use perl for UTF-8-safe
  # handling so the em-dash never gets sliced mid-rune. Default to the name.
  title="$(grep -m1 '^//' "$f" | perl -CSD -pe 's{^//+\s*}{}; s{\s*(?:\x{2014}|--|-)\s+earmuff\b.*$}{}i; s{\s*\x{2014}.*$}{}' || true)"
  [ -n "$title" ] || title="${name%.ear}"
  esc_title="$(printf '%s' "$title" | perl -CSD -pe 's/\\/\\\\/g; s/"/\\"/g')"
  [ $first -eq 1 ] || manifest="$manifest,"
  first=0
  manifest="$manifest{\"file\":\"$name\",\"title\":\"$esc_title\"}"
done
manifest="$manifest]"

printf '%s\n' "$manifest" > "$dest_dir/manifest.json"
echo "playground examples: $(ls "$dest_dir"/*.ear | wc -l | tr -d ' ') files -> $dest_dir"
