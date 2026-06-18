import * as cp from "child_process";
import * as crypto from "crypto";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as vscode from "vscode";

// Live sheet-music preview. Renders the active .ear buffer to SVG via the
// earmuff CLI (`earmuff -svg OUT.svg SOURCE.ear`) and displays it inline in a
// webview beside the editor, re-rendering (debounced) as the user types. SVG
// (rather than PDF) keeps the webview lightweight — no PDF.js, so first paint
// is fast.

const DEBOUNCE_MS = 600;

interface PreviewState {
  panel: vscode.WebviewPanel;
  // fsPath of the .ear document currently being previewed.
  docPath: string;
  // Temp files we own and must clean up on dispose.
  tempEar: string;
  tempSvg: string;
  // Pending debounce timer, if any.
  timer: NodeJS.Timeout | undefined;
  disposables: vscode.Disposable[];
  // Whether the webview has signalled it's ready to receive content.
  ready: boolean;
  // The most recent successful SVG, replayed once the webview is ready.
  lastSvg: string | undefined;
}

let state: PreviewState | undefined;

let output: vscode.OutputChannel | undefined;
function log(msg: string): void {
  if (!output) {
    output = vscode.window.createOutputChannel("earmuff sheet preview");
  }
  output.appendLine(msg);
}

function cliPath(): string {
  return vscode.workspace
    .getConfiguration("earmuff")
    .get<string>("cli.path", "earmuff");
}

// lilypondArgs returns ["-lilypond", <path>] when the user configured a
// non-default lilypond path, otherwise an empty array.
function lilypondArgs(): string[] {
  const p = vscode.workspace
    .getConfiguration("earmuff")
    .get<string>("lilypond.path", "lilypond")
    .trim();
  if (p && p !== "lilypond") {
    return ["-lilypond", p];
  }
  return [];
}

// renderSvg runs the earmuff CLI to render src -> outSvg. Resolves with the
// exit code and any captured stderr; never rejects.
function renderSvg(
  src: string,
  outSvg: string,
  cwd: string
): Promise<{ code: number; stderr: string }> {
  const cli = cliPath();
  const args = [...lilypondArgs(), "-svg", outSvg, src];
  return new Promise((resolve) => {
    let stderr = "";
    const proc = cp.spawn(cli, args, { cwd });
    proc.stdout.on("data", () => {
      /* discard CLI stdout; only stderr is useful here */
    });
    proc.stderr.on("data", (d) => {
      stderr += d.toString();
    });
    proc.on("error", (err) => {
      resolve({
        code: -1,
        stderr:
          `Could not run "${cli}". Install it with ` +
          "`go install github.com/poolpOrg/earmuff/cmd/earmuff@latest` " +
          `or set "earmuff.cli.path".\n\n${err.message}`,
      });
    });
    proc.on("close", (code) => {
      resolve({ code: code ?? 0, stderr });
    });
  });
}

function activeEarEditor(): vscode.TextEditor | undefined {
  const editor = vscode.window.activeTextEditor;
  if (editor && editor.document.languageId === "earmuff") {
    return editor;
  }
  return undefined;
}

function nonce(): string {
  return crypto.randomBytes(16).toString("hex");
}

function webviewHtml(
  webview: vscode.Webview,
  context: vscode.ExtensionContext
): string {
  const viewerUri = webview.asWebviewUri(
    vscode.Uri.joinPath(context.extensionUri, "media", "sheet-viewer.js")
  );
  const n = nonce();
  const csp = [
    `default-src 'none'`,
    // Injected SVG markup (no scripts) needs inline styles and data images.
    `img-src ${webview.cspSource} data:`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    `font-src ${webview.cspSource} data:`,
    `script-src 'nonce-${n}'`,
  ].join("; ");

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8" />
<meta http-equiv="Content-Security-Policy" content="${csp}" />
<meta name="viewport" content="width=device-width, initial-scale=1.0" />
<title>earmuff sheet preview</title>
<style>
  body {
    margin: 0;
    padding: 0;
    background: var(--vscode-editor-background);
    color: var(--vscode-editor-foreground);
    font-family: var(--vscode-font-family);
  }
  #status {
    padding: 8px 12px;
    font-size: 12px;
    opacity: 0.7;
    display: none;
  }
  #error {
    display: none;
    margin: 12px;
    padding: 12px;
    border: 1px solid var(--vscode-inputValidation-errorBorder, #be1100);
    background: var(--vscode-inputValidation-errorBackground, rgba(190, 17, 0, 0.1));
    color: var(--vscode-editor-foreground);
    white-space: pre-wrap;
    font-family: var(--vscode-editor-font-family, monospace);
    font-size: 12px;
    border-radius: 4px;
  }
  #sheet {
    padding: 12px;
    display: flex;
    justify-content: center;
  }
  /* The engraved page renders on white like paper. lilypond paints with
     fill="currentColor", so force color:#000 — otherwise the notation inherits
     the (dark-theme) editor foreground and is nearly invisible. */
  #sheet svg {
    background: #fff;
    color: #000;
    box-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
    max-width: 100%;
    height: auto;
    padding: 8px;
  }
</style>
</head>
<body>
  <div id="status"></div>
  <pre id="error"></pre>
  <div id="sheet"></div>
  <script nonce="${n}" src="${viewerUri}"></script>
</body>
</html>`;
}

// renderInto renders the given .ear document's live buffer into the panel.
async function renderInto(
  doc: vscode.TextDocument,
  st: PreviewState
): Promise<void> {
  try {
    fs.writeFileSync(st.tempEar, doc.getText(), "utf8");
  } catch (err) {
    st.panel.webview.postMessage({
      type: "error",
      message: `Failed to write temp source: ${String(err)}`,
    });
    return;
  }
  const cwd = path.dirname(doc.uri.fsPath) || os.tmpdir();
  const { code, stderr } = await renderSvg(st.tempEar, st.tempSvg, cwd);
  // The panel may have been disposed while the CLI ran.
  if (state !== st) {
    return;
  }
  if (code !== 0) {
    st.panel.webview.postMessage({
      type: "error",
      message:
        stderr.trim() ||
        `earmuff exited with code ${code}. Is lilypond installed and on PATH?`,
    });
    return;
  }
  let svg: string;
  try {
    svg = fs.readFileSync(st.tempSvg, "utf8");
  } catch (err) {
    st.panel.webview.postMessage({
      type: "error",
      message: `Rendered SVG could not be read: ${String(err)}`,
    });
    return;
  }
  // Cache it and post only once the webview has signalled ready; otherwise the
  // message races the script load and is dropped (blank panel). The "ready"
  // handler replays lastSvg.
  st.lastSvg = svg;
  log(`[preview] rendered ${svg.length} bytes; ready=${st.ready}`);
  if (st.ready) {
    st.panel.webview.postMessage({ type: "svg", svg });
  }
}

function scheduleRender(doc: vscode.TextDocument, st: PreviewState): void {
  if (st.timer) {
    clearTimeout(st.timer);
  }
  st.timer = setTimeout(() => {
    st.timer = undefined;
    void renderInto(doc, st);
  }, DEBOUNCE_MS);
}

// switchTo repoints an existing preview at a different .ear document.
function switchTo(doc: vscode.TextDocument, st: PreviewState): void {
  st.docPath = doc.uri.fsPath;
  st.panel.title = `Sheet: ${path.basename(doc.uri.fsPath)}`;
  if (st.timer) {
    clearTimeout(st.timer);
    st.timer = undefined;
  }
  void renderInto(doc, st);
}

export function showSheetPreview(context: vscode.ExtensionContext): void {
  const editor = activeEarEditor();
  if (!editor) {
    vscode.window.showErrorMessage("earmuff: no active .ear file.");
    return;
  }
  const doc = editor.document;

  // Reuse an existing panel if one is open.
  if (state) {
    state.panel.reveal(vscode.ViewColumn.Beside, true);
    if (state.docPath !== doc.uri.fsPath) {
      switchTo(doc, state);
    } else {
      void renderInto(doc, state);
    }
    return;
  }

  const panel = vscode.window.createWebviewPanel(
    "earmuffSheetPreview",
    `Sheet: ${path.basename(doc.uri.fsPath)}`,
    { viewColumn: vscode.ViewColumn.Beside, preserveFocus: true },
    {
      enableScripts: true,
      retainContextWhenHidden: true,
      localResourceRoots: [
        vscode.Uri.joinPath(context.extensionUri, "media"),
      ],
    }
  );

  const base = path.join(
    os.tmpdir(),
    `earmuff-preview-${process.pid}-${Date.now()}`
  );
  const st: PreviewState = {
    panel,
    docPath: doc.uri.fsPath,
    tempEar: `${base}.ear`,
    tempSvg: `${base}.svg`,
    timer: undefined,
    disposables: [],
    ready: false,
    lastSvg: undefined,
  };
  state = st;

  panel.webview.html = webviewHtml(panel.webview, context);

  // Re-render the previewed doc as it changes.
  // Webview -> extension messages: "ready" (replay the latest SVG, avoiding the
  // load race) and "log" (diagnostics into the output channel).
  st.disposables.push(
    panel.webview.onDidReceiveMessage((m: { type?: string; message?: string }) => {
      if (state !== st || !m) {
        return;
      }
      if (m.type === "ready") {
        st.ready = true;
        log("[webview] ready");
        if (st.lastSvg) {
          panel.webview.postMessage({ type: "svg", svg: st.lastSvg });
        }
      } else if (m.type === "log") {
        log(`[webview] ${m.message}`);
      }
    })
  );

  st.disposables.push(
    vscode.workspace.onDidChangeTextDocument((e) => {
      if (state === st && e.document.uri.fsPath === st.docPath) {
        scheduleRender(e.document, st);
      }
    })
  );

  // Follow the active editor to a different .ear file.
  st.disposables.push(
    vscode.window.onDidChangeActiveTextEditor((ed) => {
      if (state !== st || !ed || ed.document.languageId !== "earmuff") {
        return;
      }
      if (ed.document.uri.fsPath !== st.docPath) {
        switchTo(ed.document, st);
      }
    })
  );

  panel.onDidDispose(() => {
    if (st.timer) {
      clearTimeout(st.timer);
      st.timer = undefined;
    }
    for (const d of st.disposables) {
      d.dispose();
    }
    st.disposables = [];
    for (const f of [st.tempEar, st.tempSvg]) {
      try {
        fs.rmSync(f, { force: true });
      } catch {
        /* best-effort cleanup */
      }
    }
    if (state === st) {
      state = undefined;
    }
  });

  // Render the current buffer immediately.
  void renderInto(doc, st);
}
