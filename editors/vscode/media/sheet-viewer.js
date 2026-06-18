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
      // lilypond SVG carries no scripts; inject it directly. Drop any width/
      // height in mm so it scales to the panel width via the CSS rule.
      sheetEl.innerHTML = msg.svg;
      const svg = sheetEl.querySelector("svg");
      if (svg) {
        svg.removeAttribute("width");
        svg.removeAttribute("height");
      }
    } else if (msg.type === "error") {
      setStatus("");
      sheetEl.replaceChildren();
      showError(msg.message || "Unknown error.");
    }
  });
})();
