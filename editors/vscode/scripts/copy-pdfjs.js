#!/usr/bin/env node
// Copy the prebuilt PDF.js viewer + worker out of node_modules into media/ so
// they can be bundled into the .vsix and loaded into the sheet-preview webview
// via webview.asWebviewUri. The extension is esbuild-bundled and cannot import
// these large ES modules into the webview at runtime, so we ship the prebuilt
// minified legacy builds as static assets instead.
"use strict";

const fs = require("fs");
const path = require("path");

const root = path.resolve(__dirname, "..");
const src = path.join(root, "node_modules", "pdfjs-dist", "legacy", "build");
const dst = path.join(root, "media", "pdfjs");

const files = ["pdf.min.mjs", "pdf.worker.min.mjs"];

fs.mkdirSync(dst, { recursive: true });
for (const f of files) {
  const from = path.join(src, f);
  const to = path.join(dst, f);
  if (!fs.existsSync(from)) {
    console.error(`copy-pdfjs: missing ${from} — run \`npm install\` first.`);
    process.exit(1);
  }
  fs.copyFileSync(from, to);
  console.log(`copy-pdfjs: ${f} -> media/pdfjs/${f}`);
}
