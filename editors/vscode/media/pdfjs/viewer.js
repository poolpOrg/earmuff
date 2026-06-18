// Sheet-preview webview viewer. Loads PDF.js (bundled, minified legacy build)
// and renders every page of a PDF supplied as a base64 data URI by the
// extension. Messages from the extension:
//   { type: "pdf",   data: <base64 string> }  -> render the PDF
//   { type: "error", message: <string> }      -> show an error banner
// The PDF.js worker URL and the module URL are injected as data-* attributes
// on the <script> tag so we don't need any inline script (CSP-friendly).
"use strict";

(function () {
  const vscode = acquireVsCodeApi();
  // NOTE: document.currentScript is null inside an ES module, so locate our
  // own <script> by the data attributes it carries.
  const self = document.querySelector("script[data-pdfjs-url]");
  const pdfjsUrl = self.getAttribute("data-pdfjs-url");
  const workerUrl = self.getAttribute("data-worker-url");

  const statusEl = document.getElementById("status");
  const errorEl = document.getElementById("error");
  const pagesEl = document.getElementById("pages");

  let pdfjsLib = null;
  let pending = null; // last base64 received before the lib finished loading
  let renderToken = 0; // bumps on every render to cancel stale renders

  function showError(message) {
    errorEl.textContent = message;
    errorEl.style.display = "block";
  }
  function clearError() {
    errorEl.style.display = "none";
    errorEl.textContent = "";
  }
  function setStatus(text) {
    statusEl.textContent = text;
    statusEl.style.display = text ? "block" : "none";
  }

  function base64ToBytes(b64) {
    const bin = atob(b64);
    const len = bin.length;
    const bytes = new Uint8Array(len);
    for (let i = 0; i < len; i++) {
      bytes[i] = bin.charCodeAt(i);
    }
    return bytes;
  }

  async function render(b64) {
    if (!pdfjsLib) {
      pending = b64;
      return;
    }
    const token = ++renderToken;
    clearError();
    setStatus("Rendering…");
    try {
      const data = base64ToBytes(b64);
      let doc;
      try {
        doc = await pdfjsLib.getDocument({ data }).promise;
      } catch (workerErr) {
        // Most commonly a worker-spawn failure under the webview CSP. Retry on
        // the main thread.
        const data2 = base64ToBytes(b64);
        doc = await pdfjsLib.getDocument({ data: data2, disableWorker: true })
          .promise;
      }
      if (token !== renderToken) {
        return;
      }
      const frag = document.createDocumentFragment();
      const scale = 1.5;
      for (let n = 1; n <= doc.numPages; n++) {
        const page = await doc.getPage(n);
        if (token !== renderToken) {
          return;
        }
        const viewport = page.getViewport({ scale });
        const canvas = document.createElement("canvas");
        canvas.className = "page";
        const ratio = window.devicePixelRatio || 1;
        canvas.width = Math.floor(viewport.width * ratio);
        canvas.height = Math.floor(viewport.height * ratio);
        canvas.style.width = viewport.width + "px";
        canvas.style.height = viewport.height + "px";
        const ctx = canvas.getContext("2d");
        ctx.scale(ratio, ratio);
        frag.appendChild(canvas);
        await page.render({ canvasContext: ctx, viewport }).promise;
        if (token !== renderToken) {
          return;
        }
      }
      if (token !== renderToken) {
        return;
      }
      pagesEl.replaceChildren(frag);
      setStatus("");
    } catch (err) {
      if (token === renderToken) {
        showError("Failed to render PDF: " + String(err && err.message ? err.message : err));
        setStatus("");
      }
    }
  }

  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || typeof msg !== "object") {
      return;
    }
    if (msg.type === "pdf") {
      render(msg.data);
    } else if (msg.type === "error") {
      setStatus("");
      pagesEl.replaceChildren();
      showError(msg.message || "Unknown error.");
    }
  });

  // Load PDF.js as an ES module. Surface any failure visibly (the webview has
  // no console the user can easily see) and report it back to the extension.
  setStatus("Loading PDF.js…");
  import(pdfjsUrl)
    .then((mod) => {
      pdfjsLib = mod;
      // Run the worker from the bundled URL. If the worker can't start under
      // the webview CSP, disableWorker falls PDF.js back to the main thread so
      // rendering still succeeds (slower, but reliable).
      try {
        pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;
      } catch (e) {
        /* ignore; covered by the worker-port fallback below */
      }
      setStatus("");
      if (pending !== null) {
        const b64 = pending;
        pending = null;
        render(b64);
      }
    })
    .catch((err) => {
      showError(
        "Failed to load PDF.js module:\n" +
          String(err && err.message ? err.message : err) +
          "\n\nurl: " + pdfjsUrl
      );
      vscode.postMessage({ type: "viewerError", message: String(err) });
    });

  vscode.postMessage({ type: "ready" });
})();
