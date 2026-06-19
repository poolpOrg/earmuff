#!/usr/bin/env bash
# Vendor the playground's third-party browser assets (Monaco editor, VexFlow)
# into website/static/playground/. These are pinned, self-hosted (the Pages CSP
# forbids CDNs) and gitignored — CI runs this on every deploy, and you can run
# it locally to preview.
#
#   bash website/scripts/playground-assets.sh
set -euo pipefail

MONACO_VERSION="0.52.2"   # last release with the classic self-hostable min/vs AMD layout
VEROVIO_VERSION="6.2.0"   # WASM music engraver (MusicXML -> SVG)
TINYSYNTH_VERSION="1.1.3"      # WebAudio GM synth (asset-free fallback)
SPESSASYNTH_LIB_VERSION="4.3.7"   # sample-based soundfont synth (primary)
SPESSASYNTH_CORE_VERSION="4.3.10" # core that spessasynth_lib imports
# GeneralUser GS as sf3 (spessasynth's bundled default GM bank); host locally.
SOUNDFONT_URL="https://github.com/spessasus/SpessaSynth/raw/master/soundfonts/GeneralUserGS.sf3"

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

echo "==> Verovio $VEROVIO_VERSION"
( cd "$work" && npm pack "verovio@$VEROVIO_VERSION" >/dev/null 2>&1 && tar xzf verovio-*.tgz )
cp "$work/package/dist/verovio-toolkit-wasm.js" "$dest/verovio.js"

echo "==> webaudio-tinysynth $TINYSYNTH_VERSION"
( cd "$work" && npm pack "webaudio-tinysynth@$TINYSYNTH_VERSION" >/dev/null 2>&1 && tar xzf webaudio-tinysynth-*.tgz )
cp "$work/package/webaudio-tinysynth.min.js" "$dest/tinysynth.js"

echo "==> spessasynth (lib $SPESSASYNTH_LIB_VERSION + core $SPESSASYNTH_CORE_VERSION)"
spessa="$dest/spessa"
mkdir -p "$spessa"
( cd "$work" && npm pack "spessasynth_lib@$SPESSASYNTH_LIB_VERSION" >/dev/null 2>&1 && tar xzf spessasynth_lib-*.tgz )
cp "$work/package/dist/index.js" "$spessa/spessasynth_lib.js"
cp "$work/package/dist/spessasynth_processor.min.js" "$spessa/spessasynth_processor.min.js"
( cd "$work" && npm pack "spessasynth_core@$SPESSASYNTH_CORE_VERSION" >/dev/null 2>&1 && tar xzf spessasynth_core-*.tgz )
cp "$work/package/dist/index.js" "$spessa/spessasynth_core.js"

echo "==> GeneralUser GS soundfont"
curl -sSL -o "$spessa/GeneralUserGS.sf3" "$SOUNDFONT_URL"

echo "==> done"
du -sh "$dest/monaco" "$dest/verovio.js" "$dest/tinysynth.js" "$spessa"
