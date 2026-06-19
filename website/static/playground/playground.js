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
  var sheetEl = $("pg-sheet");
  var playBtn = $("pg-play");
  var stopBtn = $("pg-stop");
  var examplesSel = $("pg-examples");
  var monitorEl = $("pg-monitor");
  var errorsEl = $("pg-errors");
  var tRewind = $("pg-t-rewind");
  var tBack = $("pg-t-back");
  var tPlay = $("pg-t-play");
  var tFwd = $("pg-t-fwd");
  var tCur = $("pg-t-cur");
  var tDur = $("pg-t-dur");
  var tSeek = $("pg-t-seek");

  // ---- State -------------------------------------------------------------
  var editor = null;
  var monacoRef = null;
  var wasmReady = false;
  var lastResult = null; // last successful compile result
  var compileTimer = null;
  var sheetRendered = false;

  var DEFAULT_SOURCE =
    'project "playground" {\n' +
    "    bpm 120; time 4 4;\n\n" +
    '    track "lead" instrument "piano" {\n' +
    "        // notes carry their octave after a caret: C^5 is C in octave 5\n" +
    "        pattern hook { bar quarter { C^5 E^5 G^5 E^5 } }\n" +
    "        repeat 2 { hook }\n" +
    "    }\n\n" +
    '    track "chords" instrument "string ensemble 1" {\n' +
    "        // a bare name with a quality is a chord\n" +
    "        for ch in Am7 Dm7 G7 Cmaj7 { bar 1 { ch } }\n" +
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
          // Note with a caret octave marker (C^, C^5, Eb^3) — never a chord.
          [/[A-G](#|b)*\^\d*/, "string.note"],
          // Chord: a root + quality or bare-digit quality (Cmaj7, Am7, C5, G7),
          // optional slash bass. Matched before the bare-note rule below.
          [/[A-G](#|b)*(maj|min|dim|aug|sus|add|m|M|\d)\w*(\/[A-G](#|b)*\d*)?/, "type.identifier"],
          // Bare note: letter + accidentals only (no caret, no quality).
          [/[A-G](#|b)*(?![A-Za-z0-9^])/, "string.note"],
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

    // Clicking a problem (panel) or an inline error jumps the cursor there.
    var jumpOnClick = function (e) {
      var el = e.target.closest("[data-line]");
      if (!el) return;
      var line = +el.getAttribute("data-line");
      var col = +el.getAttribute("data-col") || 1;
      editor.revealLineInCenter(line);
      editor.setPosition({ lineNumber: line, column: col });
      editor.focus();
    };
    problemsEl.addEventListener("click", jumpOnClick);
    errorsEl.addEventListener("click", jumpOnClick);
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
      if (activeTab() === "sheet") renderSheet();
      if (live) {
        // Edited mid-playback: fold the change into the running scheduler so
        // it keeps playing from the current spot with the new notes/tempo.
        relinkLive(res);
      } else {
        setStatus(statusLine(res), "ok");
        playBtn.disabled = false;
      }
    } else {
      lastResult = null;
      if (!live) {
        playBtn.disabled = true;
        setStatus(errorCount(res) + " error(s)", "err");
      }
      // If a live edit broke the source, keep playing the last good version
      // and surface the error in the inline buffer (renderDiagnostics already did).
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

    // Inline error buffer (errors only): collapsed when the source is clean.
    var errorDiags = diags.filter(function (d) { return d.severity === "error"; });
    if (!errorDiags.length) {
      errorsEl.hidden = true;
      errorsEl.innerHTML = "";
    } else {
      errorsEl.hidden = false;
      errorsEl.innerHTML = "";
      errorDiags.forEach(function (d) {
        var row = document.createElement("div");
        row.className = "pg-err-row";
        if (d.line) {
          row.setAttribute("data-line", d.line);
          row.setAttribute("data-col", d.column || 1);
        }
        var loc = d.line ? d.line + ":" + (d.column || 1) : "—";
        row.innerHTML =
          '<span class="pg-err-loc">' + loc + "</span>" +
          "<span>" + escapeHtml(d.message) + "</span>";
        errorsEl.appendChild(row);
      });
    }
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
  // Sheet music — real engraving via Verovio (MusicXML -> SVG), lazy-loaded.
  // The MusicXML comes from the Go emitter (res.musicxml), so the notation
  // (rests, ties, beaming, clefs, multi-staff) is correct, not hand-drawn.
  // =======================================================================
  var verovioToolkit = null;
  var verovioLoading = false;

  function renderSheet() {
    if (!lastResult) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Compile something first.</div>';
      return;
    }
    if (sheetRendered) return;
    if (!lastResult.musicxml) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Nothing to engrave.</div>';
      return;
    }
    sheetEl.innerHTML = '<div class="pg-sheet-note">Engraving…</div>';
    ensureVerovio(function (tk) {
      if (!tk) {
        sheetEl.innerHTML =
          '<div class="pg-sheet-note">In-browser engraving unavailable. ' +
          "Download the LilyPond source and run <code>lilypond</code> for a full score.</div>";
        return;
      }
      drawSheet(tk, lastResult);
      sheetRendered = true;
    });
  }

  // Verovio ships as UMD. Its AMD branch declares node:fs / node:crypto as
  // dependencies, which Monaco's AMD loader can't resolve in the browser — so
  // we must NOT load it through the loader. Inject a plain <script> with the
  // global AMD `define` temporarily shadowed, forcing the UMD global branch
  // (which sets window.verovio and needs no node modules). Its wasm then
  // initializes asynchronously — wait for onRuntimeInitialized.
  function ensureVerovio(cb) {
    if (verovioToolkit) return cb(verovioToolkit);
    if (verovioLoading) return;
    verovioLoading = true;

    var savedDefine = window.define;
    window.define = undefined; // hide AMD so Verovio's UMD takes the global path
    var s = document.createElement("script");
    s.src = BASE + "/verovio.js";
    s.onload = function () {
      window.define = savedDefine; // restore Monaco's AMD loader
      var v = window.verovio;
      if (!v || !v.module) { verovioLoading = false; cb(null); return; }
      var ready = function () {
        try {
          verovioToolkit = new v.toolkit();
          setVerovioOptions(verovioToolkit);
        } catch (e) { verovioToolkit = null; }
        verovioLoading = false;
        cb(verovioToolkit);
      };
      if (v.module.calledRun) ready();
      else v.module.onRuntimeInitialized = ready;
    };
    s.onerror = function () {
      window.define = savedDefine;
      verovioLoading = false;
      cb(null);
    };
    document.head.appendChild(s);
  }

  function setVerovioOptions(tk) {
    var dark = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches;
    tk.setOptions({
      pageWidth: Math.max(800, (sheetEl.clientWidth - 24) * 2),
      pageHeight: 60000,
      scale: 45,
      adjustPageHeight: true,
      breaks: "auto",
      footer: "none",
      header: "none",
    });
  }

  function drawSheet(tk, res) {
    try {
      setVerovioOptions(tk);
      tk.loadData(res.musicxml);
      var n = tk.getPageCount() || 1;
      var svg = "";
      for (var p = 1; p <= n; p++) svg += tk.renderToSVG(p);
      sheetEl.innerHTML = svg;
    } catch (e) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Could not engrave this passage.</div>';
    }
  }

  // =======================================================================
  // Playback. Primary synth is spessasynth (sample-based General MIDI via a
  // soundfont, loaded through the module bridge in the page). If that fails to
  // load we fall back to webaudio-tinysynth (synthesized GM, no assets). Both
  // are wrapped in a small adapter exposing noteOn/noteOff/program/allOff with
  // absolute-time scheduling, so play() doesn't care which is in use.
  // =======================================================================
  var audioCtx = null;
  var synthAdapter = null;
  var synthLoading = false;
  var stopTimer = null;

  function ticksToSeconds(ticks, bpm) {
    var ppq = (lastResult && lastResult.ppq) || 960;
    return (ticks / ppq) * (60 / (bpm || 120));
  }

  function statusLine(res) {
    var secs = ticksToSeconds(res.durationTicks, res.bpm);
    return '"' + res.project + '" — ' + res.trackCount + " track" +
      (res.trackCount === 1 ? "" : "s") + ", " + res.eventCount +
      " events, " + secs.toFixed(1) + "s";
  }

  function ensureAudio() {
    if (!audioCtx) {
      var AC = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AC();
    }
    if (audioCtx.state === "suspended") audioCtx.resume();
  }

  // ensureSynth resolves an adapter, trying spessasynth first then tinysynth.
  function ensureSynth(cb) {
    if (synthAdapter) return cb(synthAdapter);
    if (synthLoading) return;
    synthLoading = true;
    ensureAudio();

    function done(a) { synthAdapter = a; synthLoading = false; cb(a); }

    if (window.__spessaReady && typeof window.__spessaCreate === "function") {
      setStatus("loading instruments…");
      window.__spessaCreate(audioCtx).then(function (synth) {
        done(spessaAdapter(synth));
      }).catch(function (err) {
        // Soundfont/worklet failed — fall back to the asset-free synth.
        console.warn("spessasynth unavailable, falling back to tinysynth:", err);
        loadTiny(done);
      });
    } else {
      loadTiny(done);
    }
  }

  function loadTiny(done) {
    window.require.config({ paths: { tinysynth: BASE + "/tinysynth" } });
    window.require(["tinysynth"], function (TinySynth) {
      var ctor = TinySynth || window.WebAudioTinySynth;
      var s = new ctor({ internalContext: 0, useReverb: 1, voices: 64 });
      s.setAudioContext(audioCtx, audioCtx.destination);
      done(tinyAdapter(s));
    }, function () {
      synthLoading = false;
      setStatus("could not load a synth", "err");
    });
  }

  // Both adapters expose IMMEDIATE (play-now) operations. We do NOT pre-schedule
  // the whole song into the synth — that can't be cancelled, which broke Stop
  // and let an old play bleed into a new one. Instead a JS scheduler (below)
  // dispatches each event when its time arrives, so Stop truly stops.
  function spessaAdapter(synth) {
    return {
      program: function (ch, pc) { synth.programChange(ch, pc & 0x7f); },
      noteOn: function (ch, key, vel) { synth.noteOn(ch, key & 0x7f, vel & 0x7f); },
      noteOff: function (ch, key) { synth.noteOff(ch, key & 0x7f); },
      allOff: function () { for (var c = 0; c < 16; c++) synth.controllerChange(c, 120, 0); },
    };
  }

  function tinyAdapter(synth) {
    return {
      program: function (ch, pc) { synth.send([0xc0 | ch, pc & 0x7f]); },
      noteOn: function (ch, key, vel) { synth.send([0x90 | ch, key & 0x7f, vel & 0x7f]); },
      noteOff: function (ch, key) { synth.send([0x80 | ch, key & 0x7f, 0]); },
      allOff: function () {
        for (var c = 0; c < 16; c++) { synth.send([0xb0 | c, 120, 0]); synth.send([0xb0 | c, 123, 0]); }
      },
    };
  }

  // ---- JS scheduler -----------------------------------------------------
  // A monotonically increasing token: every play() bumps it, and the running
  // loop bails the instant the token changes, so a new play (or stop) cancels
  // the old one cleanly even across the async synth-load.
  var playToken = 0;
  var schedTimer = null;
  var LOOKAHEAD = 25; // ms; dispatch slightly ahead so timing stays tight

  // `live` holds the in-flight playback state so an edit can be re-rendered
  // into it without restarting (see relinkLive). null when not playing.
  var live = null;

  function nowMs() { return (window.performance && performance.now()) || Date.now(); }

  // queueFromResult builds a time-sorted dispatch list (ms from start) for a
  // compiled result at the given seconds-per-tick.
  function queueFromResult(res, secPerTick) {
    var q = res.events.map(function (e) { return { ms: e.t * secPerTick * 1000, e: e }; });
    q.sort(function (a, b) { return a.ms - b.ms; });
    return q;
  }

  // posMs returns the current playback position in ms (works while paused).
  function posMs() {
    if (!live) return 0;
    return live.paused ? live.pausedAt : nowMs() - live.startMs;
  }

  function queueDuration(q) { return q.length ? q[q.length - 1].ms : 0; }

  // runLoop drives the scheduler; callable to (re)start after a seek/resume.
  function runLoop(myToken) {
    if (schedTimer) { clearTimeout(schedTimer); schedTimer = null; }
    function tick() {
      if (!live || live.token !== myToken || live.paused) return;
      var now = nowMs() - live.startMs;
      while (live.i < live.queue.length && live.queue[live.i].ms <= now + LOOKAHEAD) {
        dispatch(live.sy, live.queue[live.i++].e);
      }
      if (live.i < live.queue.length) {
        schedTimer = setTimeout(tick, 10);
      } else {
        // Done: let tails ring, then reset the UI if still our turn.
        schedTimer = setTimeout(function () {
          if (live && live.token === myToken) finishPlayback();
        }, 400);
      }
    }
    tick();
  }

  function play() {
    stop();                 // cancel any current playback first
    // Flush any pending debounced compile so we play the CURRENT editor
    // content, not the previously compiled piece (e.g. just-switched example).
    if (compileTimer) { clearTimeout(compileTimer); compileTimer = null; compileNow(); }
    if (!lastResult) return;
    ensureAudio();
    var myToken = ++playToken;

    playBtn.disabled = true;
    stopBtn.disabled = false;
    setStatus("loading instruments…");

    ensureSynth(function (sy) {
      if (myToken !== playToken) return; // superseded while loading
      var res = lastResult;
      var secPerTick = (60 / (res.bpm || 120)) / res.ppq;
      var queue = queueFromResult(res, secPerTick);

      live = {
        token: myToken,
        sy: sy,
        startMs: nowMs(),
        secPerTick: secPerTick,
        queue: queue,
        duration: queueDuration(queue),
        i: 0,
        paused: false,
        pausedAt: 0,
      };
      sendPrograms(sy, res);   // set each track's instrument up front

      monitorReset();
      setStatus("playing…", "ok");
      transportEnable(true);
      startTransportUI();
      runLoop(myToken);
    });
  }

  function sendPrograms(sy, res) {
    (res.tracks || []).forEach(function (tr) {
      if (tr.channel !== 9) sy.program(tr.channel & 0x0f, tr.program);
    });
  }

  // relinkLive swaps a freshly compiled result into the running playback
  // without restarting: it rebuilds the queue, re-sends instruments, and skips
  // the cursor past events already due, keeping the current position. A BPM
  // change retimes the remainder from where we are now.
  function relinkLive(res) {
    if (!live) return;
    live.secPerTick = (60 / (res.bpm || 120)) / res.ppq;
    live.queue = queueFromResult(res, live.secPerTick);
    live.duration = queueDuration(live.queue);
    var at = posMs();
    var i = 0;
    while (i < live.queue.length && live.queue[i].ms <= at) i++;
    live.i = i;
    sendPrograms(live.sy, res); // apply instrument changes immediately
    updateTransport();
  }

  // ---- Transport: pause / resume / seek ---------------------------------
  function pauseToggle() {
    if (!live) { play(); return; } // nothing playing -> start
    if (live.paused) {
      // resume: re-anchor the clock so position continues from pausedAt.
      live.startMs = nowMs() - live.pausedAt;
      live.paused = false;
      if (live.sy) sendPrograms(live.sy, lastResult); // restore instruments
      tPlay.textContent = "⏸";
      startTransportUI();
      runLoop(live.token);
    } else {
      live.pausedAt = nowMs() - live.startMs;
      live.paused = true;
      if (schedTimer) { clearTimeout(schedTimer); schedTimer = null; }
      if (live.sy) live.sy.allOff(); // silence held notes while paused
      clearHighlights();
      tPlay.textContent = "▶";
    }
  }

  function seekTo(ms) {
    if (!live) return;
    ms = Math.max(0, Math.min(ms, live.duration));
    if (live.sy) live.sy.allOff();       // kill anything ringing
    clearHighlights();
    var i = 0;
    while (i < live.queue.length && live.queue[i].ms < ms) i++;
    live.i = i;
    if (live.paused) {
      live.pausedAt = ms;
    } else {
      live.startMs = nowMs() - ms;
      if (live.sy) sendPrograms(live.sy, lastResult); // right instruments after a jump
      runLoop(live.token);
    }
    updateTransport();
  }

  function dispatch(sy, e) {
    var ch = e.ch & 0x0f;
    if (e.kind === 0 && e.vel > 0) {
      sy.noteOn(ch, e.key, e.vel);
      monitorPush(e);          // show it in the live buffer
      lineOn(e.line);          // light its source line
    } else if (e.kind === 1 || (e.kind === 0 && e.vel === 0)) {
      sy.noteOff(ch, e.key);
      lineOff(e.line);         // dim its source line when the note ends
    } else if (e.kind === 5) {
      sy.program(ch, e.prog);
    }
  }

  function finishPlayback() {
    live = null;
    if (synthAdapter) synthAdapter.allOff();
    clearHighlights();
    stopTransportUI();
    transportEnable(false);
    stopBtn.disabled = true;
    playBtn.disabled = !lastResult;
    if (lastResult) setStatus(statusLine(lastResult), "ok");
  }

  function stop() {
    playToken++;            // invalidate any running loop
    live = null;
    if (schedTimer) { clearTimeout(schedTimer); schedTimer = null; }
    if (synthAdapter) synthAdapter.allOff();
    clearHighlights();
    stopTransportUI();
    transportEnable(false);
    stopBtn.disabled = true;
    playBtn.disabled = !lastResult;
    if (lastResult) setStatus(statusLine(lastResult), "ok");
  }

  // ---- Transport UI -----------------------------------------------------
  var transportRAF = null;
  var seeking = false; // true while the user drags the slider

  function fmtTime(ms) {
    var s = Math.max(0, Math.round(ms / 1000));
    return Math.floor(s / 60) + ":" + ("0" + (s % 60)).slice(-2);
  }

  function transportEnable(on) {
    [tRewind, tBack, tPlay, tFwd, tSeek].forEach(function (el) { el.disabled = !on; });
    tPlay.textContent = on ? "⏸" : "▶";
    if (!on) {
      tSeek.value = 0; tCur.textContent = "0:00"; tDur.textContent = "0:00";
    }
  }

  function updateTransport() {
    if (!live) return;
    tDur.textContent = fmtTime(live.duration);
    if (!seeking) {
      var p = live.duration ? (posMs() / live.duration) * 1000 : 0;
      tSeek.value = String(Math.max(0, Math.min(1000, p)));
    }
    tCur.textContent = fmtTime(posMs());
    tPlay.textContent = live.paused ? "▶" : "⏸";
  }

  function startTransportUI() {
    stopTransportUI();
    (function frame() {
      if (!live) return;
      updateTransport();
      transportRAF = requestAnimationFrame(frame);
    })();
  }
  function stopTransportUI() {
    if (transportRAF) { cancelAnimationFrame(transportRAF); transportRAF = null; }
  }

  function wireTransport() {
    tPlay.addEventListener("click", pauseToggle);
    tRewind.addEventListener("click", function () { if (live) seekTo(0); });
    tBack.addEventListener("click", function () { if (live) seekTo(posMs() - 5000); });
    tFwd.addEventListener("click", function () { if (live) seekTo(posMs() + 5000); });
    // Slider: scrub live; the rAF updater won't fight while seeking is true.
    tSeek.addEventListener("input", function () {
      if (!live) return;
      seeking = true;
      tCur.textContent = fmtTime((+tSeek.value / 1000) * live.duration);
    });
    tSeek.addEventListener("change", function () {
      if (!live) { seeking = false; return; }
      seekTo((+tSeek.value / 1000) * live.duration);
      seeking = false;
    });
  }

  // ---- Live event monitor -----------------------------------------------
  var KIND_PC = ["C", "C#", "D", "D#", "E", "F", "F#", "G", "G#", "A", "A#", "B"];
  function pitchName(key) { return KIND_PC[key % 12] + "^" + (Math.floor(key / 12) - 1); }

  function monitorReset() {
    monitorEl.hidden = false;
    monitorEl.innerHTML = "";
  }
  function monitorPush(e) {
    var secs = ticksToSeconds(e.t, lastResult ? lastResult.bpm : 120);
    var name = e.ch === 9 ? "drum " + e.key : pitchName(e.key);
    var row = document.createElement("div");
    row.className = "pg-ev";
    row.innerHTML =
      '<span class="pg-ev-t">' + secs.toFixed(2).padStart(6) + "s</span>  " +
      '<span class="pg-ev-trk">trk' + e.track + "</span>  " +
      escapeHtml(name).padEnd(6) + "  v" + (e.vel || 0);
    monitorEl.appendChild(row);
    // Keep it light: cap the DOM and auto-scroll to the newest line.
    while (monitorEl.childNodes.length > 80) monitorEl.removeChild(monitorEl.firstChild);
    monitorEl.scrollTop = monitorEl.scrollHeight;
  }

  // ---- Source-line highlighting -----------------------------------------
  // Reference-count active notes per line so a line stays lit until all of its
  // currently-sounding notes have ended; render the lit set as Monaco decorations.
  var lineCounts = {};          // line -> active note count
  var lineDecorations = [];     // current Monaco decoration ids

  function lineOn(line) {
    if (!line) return;
    lineCounts[line] = (lineCounts[line] || 0) + 1;
    renderHighlights();
  }
  function lineOff(line) {
    if (!line || !lineCounts[line]) return;
    if (--lineCounts[line] <= 0) delete lineCounts[line];
    renderHighlights();
  }
  function clearHighlights() {
    lineCounts = {};
    renderHighlights();
  }
  function renderHighlights() {
    if (!editor || !monacoRef) return;
    var decos = Object.keys(lineCounts).map(function (l) {
      var n = +l;
      return {
        range: new monacoRef.Range(n, 1, n, 1),
        options: {
          isWholeLine: true,
          className: "pg-playing-line",
          linesDecorationsClassName: "pg-playing-glyph",
        },
      };
    });
    lineDecorations = editor.deltaDecorations(lineDecorations, decos);
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
    return t ? t.getAttribute("data-tab") : "sheet";
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

  // =======================================================================
  // Import: a .mid -> .ear (via the WASM earmuffImport)
  // =======================================================================
  function wireImport() {
    var input = $("pg-import-input");
    var drop = $("pg-drop");

    $("pg-import").addEventListener("click", function () { input.click(); });
    input.addEventListener("change", function () {
      if (input.files && input.files[0]) importFile(input.files[0]);
      input.value = ""; // allow re-importing the same file
    });

    // Full-window drag-and-drop. Counting dragenter/dragleave is unreliable
    // (child elements fire unbalanced events), so instead: show the overlay
    // while dragover keeps firing, and hide it shortly after it stops — which
    // also covers the drag leaving the window. drop always hides immediately.
    var hideTimer = null;
    window.addEventListener("dragover", function (e) {
      if (!hasFiles(e)) return;
      e.preventDefault();
      drop.hidden = false;
      if (hideTimer) clearTimeout(hideTimer);
      hideTimer = setTimeout(function () { drop.hidden = true; }, 120);
    });
    window.addEventListener("drop", function (e) {
      if (hideTimer) clearTimeout(hideTimer);
      drop.hidden = true;
      if (!hasFiles(e)) return;
      e.preventDefault();
      var f = e.dataTransfer.files[0];
      if (f) importFile(f);
    });
  }

  function hasFiles(e) {
    return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], "Files") >= 0;
  }

  function importFile(file) {
    var name = file.name || "";
    // .ear is source — load it straight into the editor. Everything else is
    // treated as a MIDI file and run through the importer.
    if (/\.ear$/i.test(name)) {
      var tr = new FileReader();
      tr.onload = function () {
        editor.setValue(String(tr.result));
        setStatus("loaded " + name, "ok");
      };
      tr.onerror = function () { setStatus("could not read " + name, "err"); };
      tr.readAsText(file);
      return;
    }

    if (!wasmReady) { setStatus("the compiler is still loading…", "err"); return; }
    setStatus("importing " + name + "…");
    var reader = new FileReader();
    reader.onload = function () {
      var bytes = new Uint8Array(reader.result);
      var b64 = bytesToB64(bytes);
      var res;
      try {
        res = JSON.parse(window.earmuffImport(b64, false /* readable */));
      } catch (err) {
        setStatus("import failed: " + err, "err");
        return;
      }
      if (!res.ok) {
        setStatus("import failed: " + (res.error || "unknown error"), "err");
        return;
      }
      editor.setValue(res.source);
      setStatus("imported " + name, "ok");
      // compileNow fires via the editor change handler.
    };
    reader.onerror = function () { setStatus("could not read " + name, "err"); };
    reader.readAsArrayBuffer(file);
  }

  function bytesToB64(bytes) {
    var bin = "";
    for (var i = 0; i < bytes.length; i++) bin += String.fromCharCode(bytes[i]);
    return btoa(bin);
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
    wireImport();
    wireTransport();
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
