/* earmuff playground — front-end app.
 *
 * Wiring:
 *   Monaco editor  --(debounced)-->  earmuffCompile(src)  (Go/WASM)
 *        |                                   |
 *        |  diagnostics as markers           +-- events  --> table + WebAudio synth
 *        |                                   +-- midiBase64 --> .mid download
 *        |                                   +-- lilypond --> text + VexFlow sheet
 *
 * Everything runs client-side; the page is served statically from GitHub Pages.
 */
(function () {
  "use strict";

  var app = document.getElementById("pg-app");
  var BASE = app.getAttribute("data-base"); // e.g. "/docs/playground"

  // ---- DOM handles -------------------------------------------------------
  var $ = function (id) { return document.getElementById(id); };
  var statusEl = $("pg-status");
  var problemsEl = $("pg-problems");
  var problemCountEl = $("pg-problem-count");
  var eventsEl = $("pg-events");
  var lilypondEl = $("pg-lilypond");
  var sheetEl = $("pg-sheet");
  var playBtn = $("pg-play");
  var stopBtn = $("pg-stop");
  var examplesSel = $("pg-examples");

  // ---- State -------------------------------------------------------------
  var editor = null;
  var monacoRef = null;
  var wasmReady = false;
  var lastResult = null; // last successful compile result
  var compileTimer = null;
  var sheetRendered = false;
  var vexLoaded = false;

  var DEFAULT_SOURCE =
    'project "playground" {\n' +
    "    bpm 120; time 4 4;\n\n" +
    '    track "lead" instrument "piano" {\n' +
    "        pattern hook { bar quarter { C E G E } }\n" +
    "        repeat 2 { hook }\n" +
    "        for each Am7 Dm7 G7 Cmaj7 { bar quarter { C } }\n" +
    "    }\n\n" +
    '    track "drums" instrument "synth drum" channel 10 {\n' +
    '        kit { hh = "closed hi-hat"; sn = "acoustic snare"; }\n' +
    "        swing 62;\n" +
    "        repeat 4 { bar 8 { hh sn hh sn hh sn hh sn } }\n" +
    "    }\n" +
    "}\n";

  function setStatus(text, cls) {
    statusEl.textContent = text;
    statusEl.className = "pg-status" + (cls ? " pg-" + cls : "");
  }

  // =======================================================================
  // Monaco: loader + earmuff language + editor
  // =======================================================================
  function bootMonaco(onReady) {
    // The AMD loader is already on the page; point it at our vendored vs/.
    window.require.config({ paths: { vs: BASE + "/monaco/vs" } });
    // Workers are same-origin; no cross-origin worker shim needed.
    window.require(["vs/editor/editor.main"], function () {
      monacoRef = window.monaco;
      registerEarmuffLanguage(monacoRef);
      onReady();
    });
  }

  function registerEarmuffLanguage(monaco) {
    monaco.languages.register({ id: "earmuff" });

    // Monarch tokenizer — ported from the VS Code TextMate grammar.
    monaco.languages.setMonarchTokensProvider("earmuff", {
      defaultToken: "",
      control: ["for", "each", "in", "if", "else", "repeat"],
      keywords: [
        "project", "track", "bar", "pattern", "section", "kit", "instrument",
        "channel", "port", "bpm", "time", "copyright", "text", "lyric",
        "marker", "cue", "on", "beat", "let", "swing", "cc", "bend", "raw",
        "range", "pressure", "program", "sysex", "then", "over",
      ],
      durations: [
        "whole", "half", "quarter", "eighth", "sixteenth",
        "thirtysecond", "sixtyfourth",
      ],
      dynamics: ["ppp", "pp", "p", "mp", "mf", "f", "ff", "fff"],
      booleans: ["true", "false"],
      tokenizer: {
        root: [
          [/\/\/.*$/, "comment"],
          [/\/\*/, "comment", "@comment"],
          [/"([^"\\]|\\.)*"/, "string"],
          [/'([^'\\]|\\.)*'/, "string"],
          // Chords/notes: a capital A-G with accidentals/quality/octave.
          [/\b[A-G](#|b)*\d?(maj|min|dim|aug|sus|add|m|M)?\d*(b\d+|#\d+)*(\/[A-G](#|b)*\d?)?\b/, "type.identifier"],
          [/\b\d+\.\d+\b/, "number.float"],
          [/\b\d+\b/, "number"],
          [/[a-zA-Z_][\w]*/, {
            cases: {
              "@control": "keyword.control",
              "@keywords": "keyword",
              "@durations": "type",
              "@dynamics": "constant",
              "@booleans": "constant",
              "@default": "identifier",
            },
          }],
          [/(==|!=|<=|>=|&&|\|\||\.\.|[<>=+\-*/!:|@~])/, "operator"],
        ],
        comment: [
          [/[^/*]+/, "comment"],
          [/\*\//, "comment", "@pop"],
          [/[/*]/, "comment"],
        ],
      },
    });

    monaco.languages.setLanguageConfiguration("earmuff", {
      comments: { lineComment: "//", blockComment: ["/*", "*/"] },
      brackets: [["{", "}"], ["[", "]"], ["(", ")"]],
      autoClosingPairs: [
        { open: "{", close: "}" },
        { open: "[", close: "]" },
        { open: "(", close: ")" },
        { open: '"', close: '"' },
      ],
    });
  }

  function createEditor(initial) {
    var dark = window.matchMedia &&
      window.matchMedia("(prefers-color-scheme: dark)").matches;
    editor = monacoRef.editor.create($("pg-editor"), {
      value: initial,
      language: "earmuff",
      theme: dark ? "vs-dark" : "vs",
      automaticLayout: true,
      minimap: { enabled: false },
      fontSize: 14,
      scrollBeyondLastLine: false,
      tabSize: 4,
    });
    editor.onDidChangeModelContent(scheduleCompile);

    // Clicking a problem jumps the cursor there.
    problemsEl.addEventListener("click", function (e) {
      var li = e.target.closest("li[data-line]");
      if (!li) return;
      var line = +li.getAttribute("data-line");
      var col = +li.getAttribute("data-col") || 1;
      editor.revealLineInCenter(line);
      editor.setPosition({ lineNumber: line, column: col });
      editor.focus();
    });
  }

  // =======================================================================
  // Compile (WASM) + render outputs
  // =======================================================================
  function scheduleCompile() {
    if (compileTimer) clearTimeout(compileTimer);
    compileTimer = setTimeout(compileNow, 280);
  }

  function compileNow() {
    if (!wasmReady || !editor) return;
    var src = editor.getValue();
    var res;
    try {
      res = JSON.parse(window.earmuffCompile(src));
    } catch (err) {
      setStatus("compiler error: " + err, "err");
      return;
    }
    renderDiagnostics(res);
    if (res.ok) {
      lastResult = res;
      sheetRendered = false; // re-render sheet lazily on tab view
      renderEvents(res);
      lilypondEl.textContent = res.lilypond || "";
      if (activeTab() === "sheet") renderSheet();
      var secs = ticksToSeconds(res.durationTicks, res.bpm);
      setStatus(
        '"' + res.project + '" — ' + res.trackCount + " track" +
        (res.trackCount === 1 ? "" : "s") + ", " + res.eventCount +
        " events, " + secs.toFixed(1) + "s",
        "ok"
      );
      playBtn.disabled = false;
    } else {
      lastResult = null;
      playBtn.disabled = true;
      setStatus(errorCount(res) + " error(s)", "err");
    }
  }

  function errorCount(res) {
    return (res.diagnostics || []).filter(function (d) {
      return d.severity === "error";
    }).length;
  }

  function renderDiagnostics(res) {
    var diags = res.diagnostics || [];
    // Editor markers.
    var markers = diags.map(function (d) {
      return {
        severity: d.severity === "error"
          ? monacoRef.MarkerSeverity.Error
          : monacoRef.MarkerSeverity.Warning,
        message: d.message,
        startLineNumber: d.line || 1,
        startColumn: d.column || 1,
        endLineNumber: d.line || 1,
        endColumn: (d.column || 1) + 1,
      };
    });
    monacoRef.editor.setModelMarkers(editor.getModel(), "earmuff", markers);

    // Problems panel.
    problemsEl.innerHTML = "";
    if (!diags.length) {
      problemsEl.innerHTML = '<li class="pg-empty">No problems. ✓</li>';
    } else {
      diags.forEach(function (d) {
        var li = document.createElement("li");
        li.className = "pg-sev-" + d.severity;
        if (d.line) {
          li.setAttribute("data-line", d.line);
          li.setAttribute("data-col", d.column || 1);
        }
        var loc = d.line ? d.line + ":" + (d.column || 1) : "—";
        li.innerHTML =
          '<span class="pg-dot">●</span>' +
          '<span class="pg-loc">' + loc + "</span>" +
          "<span>" + escapeHtml(d.message) + "</span>";
        problemsEl.appendChild(li);
      });
    }
    var errs = errorCount(res);
    problemCountEl.textContent = diags.length ? String(diags.length) : "";
    problemCountEl.className = "pg-badge" + (errs ? " pg-has-err" : "");
  }

  var KIND = ["on", "off", "cc", "bend", "press", "prog", "meta", "sysex"];
  function renderEvents(res) {
    var evs = res.events || [];
    var rows = evs.slice(0, 2000).map(function (e) {
      return "<tr><td>" + e.t + "</td><td>" + e.track + "</td><td>" +
        (KIND[e.kind] || e.kind) + "</td><td>" + e.ch + "</td><td>" +
        (e.key || "") + "</td><td>" + (e.vel || "") + "</td></tr>";
    }).join("");
    var more = evs.length > 2000
      ? '<div class="pg-sheet-note">… ' + (evs.length - 2000) + " more events</div>"
      : "";
    eventsEl.innerHTML =
      "<table><thead><tr><th>tick</th><th>trk</th><th>kind</th><th>ch</th>" +
      "<th>key</th><th>vel</th></tr></thead><tbody>" + rows + "</tbody></table>" + more;
  }

  // =======================================================================
  // Sheet music (VexFlow, lazy)
  // =======================================================================
  function renderSheet() {
    if (!lastResult) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Compile something first.</div>';
      return;
    }
    if (sheetRendered) return;
    ensureVexFlow(function (ok) {
      if (!ok) {
        sheetEl.innerHTML =
          '<div class="pg-sheet-note">In-browser engraving unavailable. ' +
          "Download the LilyPond source and run <code>lilypond</code> for a full score.</div>";
        return;
      }
      drawSheet(lastResult);
      sheetRendered = true;
    });
  }

  // VexFlow ships as UMD. Monaco's AMD loader is already on the page, so a
  // plain <script> would hit the define.amd branch and never set window.Vex.
  // Load it as an AMD module through the same loader instead.
  function ensureVexFlow(cb) {
    if (vexLoaded) return cb(!!window.Vex);
    window.require.config({ paths: { vexflow: BASE + "/vexflow" } });
    window.require(["vexflow"], function (mod) {
      window.Vex = window.Vex || mod;
      vexLoaded = true;
      cb(!!window.Vex);
    }, function () { vexLoaded = true; cb(false); });
  }

  // Draw a single-staff melodic reduction of track 0 from the event stream.
  function drawSheet(res) {
    sheetEl.innerHTML = "";
    var VF = window.Vex && window.Vex.Flow;
    if (!VF) return;
    var notes = noteEventsForTrack(res, 0);
    if (!notes.length) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Track 1 has no pitched notes to engrave.</div>';
      return;
    }
    // A lightweight, readable reduction: render up to ~32 quarter slots.
    var div = document.createElement("div");
    sheetEl.appendChild(div);
    var renderer = new VF.Renderer(div, VF.Renderer.Backends.SVG);
    var width = Math.max(560, sheetEl.clientWidth - 24);
    renderer.resize(width, 160);
    var ctx = renderer.getContext();
    var stave = new VF.Stave(8, 16, width - 24);
    stave.addClef("treble").addTimeSignature(res.timeBeats + "/" + res.timeUnit);
    stave.setContext(ctx).draw();

    var vfNotes = notes.slice(0, 16).map(function (n) {
      return new VF.StaveNote({
        keys: [keyToVexKey(n.key)],
        duration: "q",
      });
    });
    try {
      VF.Formatter.FormatAndDraw(ctx, stave, vfNotes);
      sheetEl.insertAdjacentHTML(
        "beforeend",
        '<div class="pg-sheet-note">A melodic reduction of track 1 ' +
        "(quarter-note approximation). Download the LilyPond source for the full score.</div>"
      );
    } catch (e) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Could not engrave this passage.</div>';
    }
  }

  function noteEventsForTrack(res, track) {
    return (res.events || []).filter(function (e) {
      return e.kind === 0 && e.track === track && e.vel > 0;
    });
  }

  var PCNAMES = ["c", "c#", "d", "d#", "e", "f", "f#", "g", "g#", "a", "a#", "b"];
  function keyToVexKey(key) {
    var pc = PCNAMES[key % 12];
    var oct = Math.floor(key / 12) - 1;
    return pc + "/" + oct;
  }

  // =======================================================================
  // Playback — WebAudio synth driven by the event stream
  // =======================================================================
  var audioCtx = null;
  var activeVoices = [];
  var stopTimer = null;

  function ticksToSeconds(ticks, bpm) {
    var ppq = (lastResult && lastResult.ppq) || 960;
    return (ticks / ppq) * (60 / (bpm || 120));
  }

  function play() {
    if (!lastResult) return;
    stop();
    if (!audioCtx) {
      var AC = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AC();
    }
    if (audioCtx.state === "suspended") audioCtx.resume();

    var res = lastResult;
    var ppq = res.ppq || 960;
    var secPerTick = (60 / (res.bpm || 120)) / ppq;
    var t0 = audioCtx.currentTime + 0.06;

    // Pair NoteOn/NoteOff per (track, channel, key) to get gated durations.
    var pending = {};
    var maxEnd = 0;
    var perc = {}; // channel 10 (index 9) -> percussion: short click
    res.events.forEach(function (e) {
      if (e.kind === 0 && e.vel > 0) {
        // NoteOn
        var k = e.track + ":" + e.ch + ":" + e.key;
        (pending[k] = pending[k] || []).push(e.t);
      } else if (e.kind === 1 || (e.kind === 0 && e.vel === 0)) {
        // NoteOff (or zero-velocity NoteOn)
        var k2 = e.track + ":" + e.ch + ":" + e.key;
        var q = pending[k2];
        if (q && q.length) {
          var onTick = q.shift();
          var start = t0 + onTick * secPerTick;
          var dur = Math.max(0.05, (e.t - onTick) * secPerTick);
          scheduleVoice(e.key, e.ch, start, dur, e.vel || 80);
          if (start + dur > maxEnd) maxEnd = start + dur;
        }
      }
    });

    playBtn.disabled = true;
    stopBtn.disabled = false;
    var ms = Math.max(200, (maxEnd - audioCtx.currentTime) * 1000 + 150);
    stopTimer = setTimeout(stop, ms);
  }

  // A small subtractive voice: drums (ch 10 == index 9) get a noise burst,
  // pitched notes get a triangle+sine blend with a short ADSR. This is the
  // built-in fallback synth; a SoundFont can replace scheduleVoice later.
  function scheduleVoice(key, ch, start, dur, vel) {
    var gainPeak = Math.min(0.28, (vel / 127) * 0.32);
    if (ch === 9) {
      // Percussion: filtered noise burst.
      var buf = noiseBuffer();
      var src = audioCtx.createBufferSource();
      src.buffer = buf;
      var bp = audioCtx.createBiquadFilter();
      bp.type = key < 45 ? "lowpass" : "highpass";
      bp.frequency.value = key < 45 ? 220 : 5000;
      var g = audioCtx.createGain();
      g.gain.setValueAtTime(gainPeak, start);
      g.gain.exponentialRampToValueAtTime(0.001, start + Math.min(dur, 0.18));
      src.connect(bp); bp.connect(g); g.connect(audioCtx.destination);
      src.start(start); src.stop(start + 0.2);
      activeVoices.push(src);
      return;
    }
    var freq = 440 * Math.pow(2, (key - 69) / 12);
    var osc1 = audioCtx.createOscillator();
    var osc2 = audioCtx.createOscillator();
    osc1.type = "triangle"; osc2.type = "sine";
    osc1.frequency.value = freq; osc2.frequency.value = freq;
    var g = audioCtx.createGain();
    var a = 0.008, r = Math.min(0.12, dur * 0.5);
    g.gain.setValueAtTime(0, start);
    g.gain.linearRampToValueAtTime(gainPeak, start + a);
    g.gain.setValueAtTime(gainPeak, start + Math.max(a, dur - r));
    g.gain.exponentialRampToValueAtTime(0.001, start + dur);
    osc1.connect(g); osc2.connect(g); g.connect(audioCtx.destination);
    osc1.start(start); osc2.start(start);
    osc1.stop(start + dur + 0.02); osc2.stop(start + dur + 0.02);
    activeVoices.push(osc1, osc2);
  }

  var _noise = null;
  function noiseBuffer() {
    if (_noise) return _noise;
    var n = audioCtx.sampleRate * 0.3;
    var b = audioCtx.createBuffer(1, n, audioCtx.sampleRate);
    var d = b.getChannelData(0);
    for (var i = 0; i < n; i++) d[i] = Math.random() * 2 - 1;
    _noise = b;
    return b;
  }

  function stop() {
    if (stopTimer) { clearTimeout(stopTimer); stopTimer = null; }
    activeVoices.forEach(function (v) { try { v.stop(); } catch (e) {} });
    activeVoices = [];
    stopBtn.disabled = true;
    playBtn.disabled = !lastResult;
  }

  // =======================================================================
  // Downloads
  // =======================================================================
  function download(name, bytesOrText, mime) {
    var blob = bytesOrText instanceof Uint8Array
      ? new Blob([bytesOrText], { type: mime })
      : new Blob([bytesOrText], { type: mime });
    var url = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url; a.download = name;
    document.body.appendChild(a); a.click(); a.remove();
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }

  function b64ToBytes(b64) {
    var bin = atob(b64);
    var out = new Uint8Array(bin.length);
    for (var i = 0; i < bin.length; i++) out[i] = bin.charCodeAt(i);
    return out;
  }

  function projectFilename(ext) {
    var name = (lastResult && lastResult.project) || "song";
    return name.replace(/[^a-z0-9_-]+/gi, "-").toLowerCase() + "." + ext;
  }

  // =======================================================================
  // Tabs
  // =======================================================================
  function activeTab() {
    var t = document.querySelector(".pg-tab-active");
    return t ? t.getAttribute("data-tab") : "problems";
  }
  function initTabs() {
    document.querySelector(".pg-tabs").addEventListener("click", function (e) {
      var btn = e.target.closest(".pg-tab");
      if (!btn) return;
      var tab = btn.getAttribute("data-tab");
      document.querySelectorAll(".pg-tab").forEach(function (b) {
        b.classList.toggle("pg-tab-active", b === btn);
      });
      document.querySelectorAll(".pg-panel").forEach(function (p) {
        p.classList.toggle("pg-panel-active", p.getAttribute("data-panel") === tab);
      });
      if (tab === "sheet") renderSheet();
    });
  }

  // =======================================================================
  // Examples + share links
  // =======================================================================
  function initExamples() {
    // The list is fetched from a manifest the build writes; fall back to none.
    fetch(BASE + "/examples/manifest.json")
      .then(function (r) { return r.ok ? r.json() : []; })
      .then(function (list) {
        if (!list || !list.length) {
          examplesSel.innerHTML = '<option value="">(starter)</option>';
          return;
        }
        examplesSel.innerHTML =
          '<option value="">(starter)</option>' +
          list.map(function (f) {
            return '<option value="' + f.file + '">' + escapeHtml(f.title) + "</option>";
          }).join("");
      })
      .catch(function () { examplesSel.innerHTML = '<option value="">(starter)</option>'; });

    examplesSel.addEventListener("change", function () {
      var file = examplesSel.value;
      if (!file) { editor.setValue(DEFAULT_SOURCE); return; }
      fetch(BASE + "/examples/" + file)
        .then(function (r) { return r.text(); })
        .then(function (src) { editor.setValue(src); });
    });
  }

  function loadFromHash() {
    var m = location.hash.match(/[#&]code=([^&]+)/);
    if (!m) return null;
    try {
      return decodeURIComponent(escape(atob(decodeURIComponent(m[1]))));
    } catch (e) { return null; }
  }

  function share() {
    var src = editor.getValue();
    var code = encodeURIComponent(btoa(unescape(encodeURIComponent(src))));
    var url = location.origin + location.pathname + "#code=" + code;
    history.replaceState(null, "", url);
    if (navigator.clipboard) {
      navigator.clipboard.writeText(url).then(function () {
        setStatus("shareable link copied to clipboard", "ok");
      });
    } else {
      setStatus("link in address bar", "ok");
    }
  }

  // =======================================================================
  // Misc helpers + wiring
  // =======================================================================
  function escapeHtml(s) {
    return String(s).replace(/[&<>"]/g, function (c) {
      return { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" }[c];
    });
  }

  function wireToolbar() {
    playBtn.addEventListener("click", play);
    stopBtn.addEventListener("click", stop);
    $("pg-share").addEventListener("click", share);

    var menu = $("pg-download-menu");
    $("pg-download").addEventListener("click", function () {
      menu.hidden = !menu.hidden;
    });
    document.addEventListener("click", function (e) {
      if (!e.target.closest(".pg-menu")) menu.hidden = true;
    });
    menu.addEventListener("click", function (e) {
      var b = e.target.closest("button[data-dl]");
      if (!b) return;
      menu.hidden = true;
      var kind = b.getAttribute("data-dl");
      if (kind === "ear") {
        download(projectFilename("ear"), editor.getValue(), "text/plain");
      } else if (!lastResult) {
        setStatus("compile successfully first", "err");
      } else if (kind === "mid") {
        download(projectFilename("mid"), b64ToBytes(lastResult.midiBase64), "audio/midi");
      } else if (kind === "ly") {
        download(projectFilename("ly"), lastResult.lilypond, "text/plain");
      }
    });
  }

  // ---- WASM bootstrap ----------------------------------------------------
  function bootWasm(onReady) {
    var go = new window.Go();
    WebAssembly.instantiateStreaming(
      fetch(BASE + "/earmuff.wasm"),
      go.importObject
    ).then(function (obj) {
      go.run(obj.instance); // registers earmuffCompile, then blocks on select{}
      wasmReady = true;
      onReady();
    }).catch(function (err) {
      setStatus("failed to load the compiler: " + err, "err");
    });
  }

  // ---- Go ----------------------------------------------------------------
  function start() {
    initTabs();
    wireToolbar();
    bootMonaco(function () {
      var initial = loadFromHash() || DEFAULT_SOURCE;
      createEditor(initial);
      initExamples();
      bootWasm(function () {
        setStatus("ready", "ok");
        compileNow();
      });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", start);
  } else {
    start();
  }
})();
