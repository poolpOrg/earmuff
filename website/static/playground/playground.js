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

  // Draw every pitched track from the event stream as its own row of staves:
  // simultaneous notes become chords, durations come from each note's gate,
  // each track gets a clef chosen from its register, and the line is broken
  // into measures. Channel-10 percussion tracks are skipped (not pitched).
  function drawSheet(res) {
    sheetEl.innerHTML = "";
    var VF = window.Vex && window.Vex.Flow;
    if (!VF) return;

    var ppq = res.ppq || 960;
    var beats = res.timeBeats || 4;
    var unit = res.timeUnit || 4;
    var ticksPerBar = ppq * (4 / unit) * beats;
    var MAX_BARS = 8;

    // Collect the pitched tracks that actually have notes.
    var parts = [];
    var trackCount = (res.tracks && res.tracks.length) || 0;
    for (var t = 0; t < trackCount; t++) {
      var info = res.tracks[t] || {};
      if (info.channel === 9) continue; // GM percussion (0-based ch 9 == MIDI 10)
      var groups = chordGroupsForTrack(res, t);
      if (!groups.length) continue;
      parts.push({
        index: t,
        name: info.name || "track " + (t + 1),
        clef: clefForGroups(groups),
        measures: groupsToMeasures(groups, ticksPerBar, MAX_BARS),
      });
    }

    if (!parts.length) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">No pitched tracks to engrave (percussion is omitted).</div>';
      return;
    }

    // Fit the available panel width; only force a wider canvas (with the panel
    // scrolling) when the panel is too narrow to engrave even one bar.
    var totalW = Math.max(320, sheetEl.clientWidth - 16);
    var rowH = 110;

    try {
      parts.forEach(function (part) {
        var label = document.createElement("div");
        label.className = "pg-sheet-part";
        label.textContent = part.name;
        sheetEl.appendChild(label);

        var div = document.createElement("div");
        sheetEl.appendChild(div);
        renderPart(VF, div, part, totalW, rowH, beats, unit);
      });
      sheetEl.insertAdjacentHTML(
        "beforeend",
        '<div class="pg-sheet-note">All pitched tracks, simultaneous notes as ' +
        "chords (durations quantized; percussion omitted). Download the LilyPond " +
        "source for the full score.</div>"
      );
    } catch (e) {
      sheetEl.innerHTML = '<div class="pg-sheet-note">Could not engrave this passage.</div>';
    }
  }

  // renderPart lays one track's measures across as many rows as needed, sizing
  // each measure by its note count so dense bars are not clipped, and wrapping
  // when a row is full. Each measure is its own non-strict Voice (tolerant of
  // bars that don't sum to an exact measure after duration quantization).
  function renderPart(VF, container, part, totalW, rowH, beats, unit) {
    var leftPad = 8;
    var clefW = 60;      // room the first measure of a row gives to clef/time
    var noteW = 42;      // approximate width budget per note/chord
    var minBarW = 110;

    // Pre-measure each bar's preferred width from its note count.
    var bars = part.measures.map(function (m) {
      return { notes: m, w: Math.max(minBarW, m.length * noteW) };
    });

    // Greedy row packing: fit as many bars as the width allows.
    var rows = [];
    var cur = [], curW = leftPad + clefW;
    bars.forEach(function (b) {
      var add = b.w;
      if (cur.length && curW + add > totalW) {
        rows.push(cur); cur = []; curW = leftPad + clefW;
      }
      cur.push(b); curW += add;
    });
    if (cur.length) rows.push(cur);

    var renderer = new VF.Renderer(container, VF.Renderer.Backends.SVG);
    renderer.resize(totalW, 12 + rows.length * rowH);
    var ctx = renderer.getContext();

    rows.forEach(function (row, ri) {
      // Distribute spare width proportionally so each row fills totalW.
      var base = leftPad + (ri === 0 ? clefW : clefW);
      var sumW = row.reduce(function (s, b) { return s + b.w; }, 0);
      var avail = totalW - leftPad - clefW - 8;
      var scale = avail / sumW;

      var x = leftPad;
      row.forEach(function (b, bi) {
        var w = bi === 0 ? Math.max(b.w * scale + clefW, minBarW + clefW)
                         : Math.max(b.w * scale, minBarW);
        var stave = new VF.Stave(x, 8 + ri * rowH, w);
        if (bi === 0) {
          stave.addClef(part.clef);
          if (ri === 0) stave.addTimeSignature(beats + "/" + unit);
        }
        stave.setContext(ctx).draw();

        var vfNotes = b.notes.map(function (g) {
          return new VF.StaveNote({
            clef: part.clef,
            keys: g.keys.map(keyToVexKey),
            duration: g.duration,
          });
        });
        if (vfNotes.length) {
          var voice = new VF.Voice({ num_beats: beats, beat_value: unit })
            .setMode(VF.Voice.Mode.SOFT);
          voice.addTickables(vfNotes);
          var fmtW = w - (bi === 0 ? clefW + 16 : 16);
          new VF.Formatter().joinVoices([voice]).format([voice], Math.max(40, fmtW));
          voice.draw(ctx, stave);
        }
        x += w;
      });
    });
  }

  // clefForGroups mirrors the LilyPond emitter: bass clef when the average
  // pitch sits below ~G#3 (MIDI 56), treble otherwise.
  function clefForGroups(groups) {
    var sum = 0, n = 0;
    groups.forEach(function (g) {
      g.keys.forEach(function (k) { sum += k; n++; });
    });
    return n && sum / n < 56 ? "bass" : "treble";
  }

  // chordGroupsForTrack pairs NoteOn/NoteOff for a track and groups notes that
  // share an onset tick into a chord, returning {tick, keys[], durTicks}.
  function chordGroupsForTrack(res, track) {
    var open = {}; // key -> onset tick
    var groupsByTick = {};
    (res.events || []).forEach(function (e) {
      if (e.track !== track) return;
      var isOn = e.kind === 0 && e.vel > 0;
      var isOff = e.kind === 1 || (e.kind === 0 && e.vel === 0);
      if (isOn) {
        open[e.key] = e.t;
        var g = groupsByTick[e.t] || (groupsByTick[e.t] = { tick: e.t, keys: [], durTicks: 0 });
        if (g.keys.indexOf(e.key) < 0) g.keys.push(e.key);
      } else if (isOff && open[e.key] != null) {
        var onset = open[e.key];
        delete open[e.key];
        var g2 = groupsByTick[onset];
        if (g2) {
          var d = e.t - onset;
          if (d > g2.durTicks) g2.durTicks = d; // chord rings as long as its longest voice
        }
      }
    });
    return Object.keys(groupsByTick)
      .map(function (k) { return groupsByTick[k]; })
      .sort(function (a, b) { return a.tick - b.tick; });
  }

  // groupsToMeasures buckets chord groups into bars and assigns each group a
  // VexFlow duration string quantized from its tick length.
  function groupsToMeasures(groups, ticksPerBar, maxBars) {
    var measures = [];
    groups.forEach(function (g) {
      var bar = Math.floor(g.tick / ticksPerBar);
      if (bar >= maxBars) return;
      (measures[bar] || (measures[bar] = [])).push({
        keys: g.keys.slice().sort(function (a, b) { return a - b; }),
        duration: ticksToVexDuration(g.durTicks, ticksPerBar),
      });
    });
    // Drop empty leading/trailing holes, keep order.
    return measures.filter(function (m) { return m && m.length; });
  }

  // Map a tick length to the nearest plain VexFlow duration (no tuplets/dots).
  function ticksToVexDuration(ticks, ticksPerBar) {
    var whole = ticksPerBar; // assumes 4/4-ish; good enough for a reduction
    var table = [
      [whole, "w"], [whole / 2, "h"], [whole / 4, "q"],
      [whole / 8, "8"], [whole / 16, "16"],
    ];
    var best = "q", bestDiff = Infinity;
    for (var i = 0; i < table.length; i++) {
      var diff = Math.abs(ticks - table[i][0]);
      if (diff < bestDiff) { bestDiff = diff; best = table[i][1]; }
    }
    return best;
  }

  var PCNAMES = ["c", "c#", "d", "d#", "e", "f", "f#", "g", "g#", "a", "a#", "b"];
  function keyToVexKey(key) {
    var pc = PCNAMES[key % 12];
    var oct = Math.floor(key / 12) - 1;
    return pc + "/" + oct;
  }

  // =======================================================================
  // Playback — WebAudioTinySynth, a General MIDI synth driven by the event
  // stream. It gives every GM program its own timbre (so a piano track and a
  // violin track actually sound different) and a proper drum kit on channel 10.
  // The synth shares our AudioContext and is loaded lazily on first Play.
  // =======================================================================
  var audioCtx = null;
  var synth = null;
  var synthLoading = false;
  var stopTimer = null;

  function ticksToSeconds(ticks, bpm) {
    var ppq = (lastResult && lastResult.ppq) || 960;
    return (ticks / ppq) * (60 / (bpm || 120));
  }

  function ensureAudio() {
    if (!audioCtx) {
      var AC = window.AudioContext || window.webkitAudioContext;
      audioCtx = new AC();
    }
    if (audioCtx.state === "suspended") audioCtx.resume();
  }

  // ensureSynth loads tinysynth (AMD, via Monaco's loader to avoid its UMD
  // taking the define branch) and binds it to our AudioContext, then calls cb.
  function ensureSynth(cb) {
    if (synth) return cb(synth);
    if (synthLoading) return;
    synthLoading = true;
    ensureAudio();
    window.require.config({ paths: { tinysynth: BASE + "/tinysynth" } });
    window.require(["tinysynth"], function (TinySynth) {
      var ctor = TinySynth || window.WebAudioTinySynth;
      synth = new ctor({ internalContext: 0, useReverb: 1, voices: 64 });
      synth.setAudioContext(audioCtx, audioCtx.destination);
      synthLoading = false;
      cb(synth);
    }, function () {
      synthLoading = false;
      setStatus("could not load the synth", "err");
    });
  }

  function play() {
    if (!lastResult) return;
    stop();
    ensureAudio();
    ensureSynth(function (sy) {
      var res = lastResult;
      var ppq = res.ppq || 960;
      var secPerTick = (60 / (res.bpm || 120)) / ppq;
      var t0 = audioCtx.currentTime + 0.08;

      // Give each track's channel its General MIDI program up front, so timbres
      // differ. Percussion (channel 9) is the GM drum kit and needs no program.
      (res.tracks || []).forEach(function (tr) {
        if (tr.channel !== 9) sy.send([0xc0 | (tr.channel & 0x0f), tr.program & 0x7f], t0);
      });

      var maxTick = res.durationTicks || 0;
      res.events.forEach(function (e) {
        var t = t0 + e.t * secPerTick;
        var ch = e.ch & 0x0f;
        if (e.kind === 0 && e.vel > 0) {
          sy.send([0x90 | ch, e.key & 0x7f, e.vel & 0x7f], t);
        } else if (e.kind === 1 || (e.kind === 0 && e.vel === 0)) {
          sy.send([0x80 | ch, e.key & 0x7f, 0], t);
        } else if (e.kind === 5) {
          sy.send([0xc0 | ch, e.prog & 0x7f], t); // in-stream program change
        }
        if (e.t > maxTick) maxTick = e.t;
      });

      playBtn.disabled = true;
      stopBtn.disabled = false;
      var ms = Math.max(300, maxTick * secPerTick * 1000 + 400);
      stopTimer = setTimeout(stop, ms);
    });
  }

  function stop() {
    if (stopTimer) { clearTimeout(stopTimer); stopTimer = null; }
    if (synth) {
      // All-sound-off + all-notes-off on every channel, immediately.
      for (var ch = 0; ch < 16; ch++) {
        synth.send([0xb0 | ch, 120, 0]);
        synth.send([0xb0 | ch, 123, 0]);
      }
    }
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
    if (!wasmReady) { setStatus("the compiler is still loading…", "err"); return; }
    setStatus("importing " + file.name + "…");
    var reader = new FileReader();
    reader.onload = function () {
      var bytes = new Uint8Array(reader.result);
      var b64 = bytesToB64(bytes);
      // Honor the faithful toggle if present; default to readable.
      var faithful = false;
      var res;
      try {
        res = JSON.parse(window.earmuffImport(b64, faithful));
      } catch (err) {
        setStatus("import failed: " + err, "err");
        return;
      }
      if (!res.ok) {
        setStatus("import failed: " + (res.error || "unknown error"), "err");
        return;
      }
      editor.setValue(res.source);
      setStatus("imported " + file.name, "ok");
      // compileNow fires via the editor change handler.
    };
    reader.onerror = function () { setStatus("could not read " + file.name, "err"); };
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
