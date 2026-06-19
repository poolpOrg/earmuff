#!/usr/bin/env bash
# Vendor the playground's third-party browser assets (Monaco editor, VexFlow)
# into website/static/playground/. These are pinned, self-hosted (the Pages CSP
# forbids CDNs) and gitignored — CI runs this on every deploy, and you can run
# it locally to preview.
#
#   bash website/scripts/playground-assets.sh
set -euo pipefail

MONACO_VERSION="0.52.2"   # last release with the classic self-hostable min/vs AMD layout
VEXFLOW_VERSION="4.2.3"
TINYSYNTH_VERSION="1.1.3" # WebAudio GM synth (asset-free playback)

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
dest="$repo_root/website/static/playground"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

echo "==> Monaco $MONACO_VERSION"
( cd "$work" && npm pack "monaco-editor@$MONACO_VERSION" >/dev/null 2>&1 && tar xzf monaco-editor-*.tgz )
vs_src="$work/package/min/vs"
vs_dst="$dest/monaco/vs"
rm -rf "$dest/monaco"
mkdir -p "$vs_dst/editor" "$vs_dst/base"
cp "$vs_src/loader.js" "$vs_dst/"
cp "$vs_src/editor/editor.main.js" "$vs_src/editor/editor.main.css" "$vs_dst/editor/"
cp -r "$vs_src/base/." "$vs_dst/base/"

echo "==> VexFlow $VEXFLOW_VERSION"
( cd "$work" && npm pack "vexflow@$VEXFLOW_VERSION" >/dev/null 2>&1 && tar xzf vexflow-*.tgz )
cp "$work/package/build/cjs/vexflow-bravura.js" "$dest/vexflow.js"

echo "==> webaudio-tinysynth $TINYSYNTH_VERSION"
( cd "$work" && npm pack "webaudio-tinysynth@$TINYSYNTH_VERSION" >/dev/null 2>&1 && tar xzf webaudio-tinysynth-*.tgz )
cp "$work/package/webaudio-tinysynth.min.js" "$dest/tinysynth.js"

echo "==> done"
du -sh "$dest/monaco" "$dest/vexflow.js" "$dest/tinysynth.js"
