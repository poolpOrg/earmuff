import * as cp from "child_process";
import * as crypto from "crypto";
import * as fs from "fs";
import * as os from "os";
import * as path from "path";
import * as vscode from "vscode";

// Live sheet-music PDF preview. Renders the active .ear buffer to a PDF via the
// earmuff CLI (`earmuff -pdf OUT.pdf SOURCE.ear`) and displays it in a webview
// beside the editor, re-rendering (debounced) as the user types.

const DEBOUNCE_MS = 600;

interface PreviewState {
  panel: vscode.WebviewPanel;
  // fsPath of the .ear document currently being previewed.
  docPath: string;
  // Temp files we own and must clean up on dispose.
  tempEar: string;
  tempPdf: string;
  // Pending debounce timer, if any.
  timer: NodeJS.Timeout | undefined;
  disposables: vscode.Disposable[];
}

let state: PreviewState | undefined;

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

// renderPdf runs the earmuff CLI to render src -> outPdf. Resolves with the
// exit code and any captured stderr; never rejects.
function renderPdf(
  src: string,
  outPdf: string,
  cwd: string
): Promise<{ code: number; stderr: string }> {
  const cli = cliPath();
  const args = [...lilypondArgs(), "-pdf", outPdf, src];
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
  const mediaRoot = vscode.Uri.joinPath(context.extensionUri, "media", "pdfjs");
  const viewerUri = webview.asWebviewUri(
    vscode.Uri.joinPath(mediaRoot, "viewer.js")
  );
  const pdfjsUri = webview.asWebviewUri(
    vscode.Uri.joinPath(mediaRoot, "pdf.min.mjs")
  );
  const workerUri = webview.asWebviewUri(
    vscode.Uri.joinPath(mediaRoot, "pdf.worker.min.mjs")
  );
  const n = nonce();
  const csp = [
    `default-src 'none'`,
    `img-src ${webview.cspSource} data: blob:`,
    `style-src ${webview.cspSource} 'unsafe-inline'`,
    // PDF.js and our viewer load as ES modules; the worker is a same-origin
    // script. nonce gates the entry <script>; the module graph it pulls in is
    // covered by script-src ${webview.cspSource}.
    `script-src ${webview.cspSource} 'nonce-${n}'`,
    `worker-src ${webview.cspSource} blob:`,
    `connect-src ${webview.cspSource} blob: data:`,
    `font-src ${webview.cspSource}`,
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
  #pages {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 12px;
  }
  canvas.page {
    background: #fff;
    box-shadow: 0 1px 6px rgba(0, 0, 0, 0.4);
    max-width: 100%;
  }
</style>
</head>
<body>
  <div id="status"></div>
  <pre id="error"></pre>
  <div id="pages"></div>
  <script
    type="module"
    nonce="${n}"
    src="${viewerUri}"
    data-pdfjs-url="${pdfjsUri}"
    data-worker-url="${workerUri}"></script>
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
  const { code, stderr } = await renderPdf(st.tempEar, st.tempPdf, cwd);
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
  let bytes: Buffer;
  try {
    bytes = fs.readFileSync(st.tempPdf);
  } catch (err) {
    st.panel.webview.postMessage({
      type: "error",
      message: `Rendered PDF could not be read: ${String(err)}`,
    });
    return;
  }
  st.panel.webview.postMessage({
    type: "pdf",
    data: bytes.toString("base64"),
  });
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
    tempPdf: `${base}.pdf`,
    timer: undefined,
    disposables: [],
  };
  state = st;

  panel.webview.html = webviewHtml(panel.webview, context);

  // Re-render the previewed doc as it changes.
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
    for (const f of [st.tempEar, st.tempPdf]) {
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
