// Sheet-preview webview viewer. Receives rendered SVG from the extension and
// displays it. Messages in:
//   { type: "svg",   svg: "<svg ...>" }   -> show the score
//   { type: "error", message: "..." }     -> show an error banner
// Messages out (for diagnostics in the extension's output channel):
//   { type: "ready" }                      -> listener installed, send content
//   { type: "log",   message: "..." }      -> progress/diagnostic line
"use strict";

(function () {
  const vscode = acquireVsCodeApi();
  const errorEl = document.getElementById("error");
  const statusEl = document.getElementById("status");
  const sheetEl = document.getElementById("sheet");

  function log(m) {
    vscode.postMessage({ type: "log", message: m });
  }
  function showError(message) {
    errorEl.textContent = message;
    errorEl.style.display = "block";
  }
  function clearError() {
    errorEl.style.display = "none";
    errorEl.textContent = "";
  }
  function setStatus(text) {
    statusEl.textContent = text || "";
    statusEl.style.display = text ? "block" : "none";
  }

  function showSvg(markup) {
    try {
      log("svg received, " + markup.length + " bytes");
      const parsed = new DOMParser().parseFromString(markup, "image/svg+xml");
      const perr = parsed.querySelector("parsererror");
      if (perr) {
        showError("SVG parse error: " + perr.textContent.slice(0, 300));
        log("parsererror");
        return;
      }
      const svg = parsed.documentElement;
      if (!svg || svg.nodeName.toLowerCase() !== "svg") {
        showError("Rendered output was not valid SVG (root=" +
          (svg && svg.nodeName) + ").");
        return;
      }
      const vb = svg.getAttribute("viewBox") || "";
      svg.removeAttribute("width");
      svg.removeAttribute("height");
      svg.setAttribute("width", "100%");
      svg.setAttribute("preserveAspectRatio", "xMidYMin meet");
      svg.style.height = "auto";
      const node = document.importNode(svg, true);
      sheetEl.replaceChildren(node);
      clearError();
      setStatus("");
      log("svg rendered, viewBox=" + vb);
    } catch (e) {
      showError("Failed to display SVG: " + String(e && e.message ? e.message : e));
      log("exception: " + String(e));
    }
  }

  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || typeof msg !== "object") {
      return;
    }
    if (msg.type === "svg") {
      showSvg(msg.svg);
    } else if (msg.type === "error") {
      setStatus("");
      sheetEl.replaceChildren();
      showError(msg.message || "Unknown error.");
    }
  });

  // Tell the extension we're ready so it can (re)send content without racing
  // the script load.
  setStatus("Waiting for render…");
  vscode.postMessage({ type: "ready" });
})();
