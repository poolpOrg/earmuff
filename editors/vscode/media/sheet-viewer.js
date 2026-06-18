// Sheet-preview webview viewer. Receives rendered SVG from the extension and
// injects it inline. Messages:
//   { type: "svg",   svg: "<svg ...>...</svg>" }  -> show the score
//   { type: "error", message: "..." }             -> show an error banner
"use strict";

(function () {
  const errorEl = document.getElementById("error");
  const statusEl = document.getElementById("status");
  const sheetEl = document.getElementById("sheet");

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

  window.addEventListener("message", (event) => {
    const msg = event.data;
    if (!msg || typeof msg !== "object") {
      return;
    }
    if (msg.type === "svg") {
      clearError();
      setStatus("");
      // Parse as SVG (not via innerHTML — that builds HTML-namespaced nodes
      // that don't render). lilypond SVG carries no scripts.
      const parsed = new DOMParser().parseFromString(
        msg.svg,
        "image/svg+xml"
      );
      const svg = parsed.documentElement;
      if (!svg || svg.nodeName.toLowerCase() !== "svg") {
        showError("Rendered output was not valid SVG.");
        return;
      }
      // lilypond sizes the page in mm; let the viewBox drive the aspect ratio
      // and scale to the panel width. Setting only CSS height:auto with no
      // width can collapse the SVG to zero height, so set width explicitly.
      svg.removeAttribute("width");
      svg.removeAttribute("height");
      svg.setAttribute("width", "100%");
      svg.setAttribute("preserveAspectRatio", "xMidYMin meet");
      svg.style.height = "auto";
      sheetEl.replaceChildren(document.importNode(svg, true));
    } else if (msg.type === "error") {
      setStatus("");
      sheetEl.replaceChildren();
      showError(msg.message || "Unknown error.");
    }
  });
})();
