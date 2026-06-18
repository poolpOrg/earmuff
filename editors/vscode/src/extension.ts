import * as cp from "child_process";
import * as fs from "fs";
import * as path from "path";
import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";
import { showSheetPreview } from "./preview";

let client: LanguageClient | undefined;
let output: vscode.OutputChannel | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  output = vscode.window.createOutputChannel("earmuff");
  context.subscriptions.push(output);

  context.subscriptions.push(
    vscode.commands.registerCommand("earmuff.compileMidi", () => compileMidi()),
    vscode.commands.registerCommand("earmuff.play", () => play()),
    vscode.commands.registerCommand("earmuff.showSheetPreview", () =>
      showSheetPreview(context)
    )
  );

  const config = vscode.workspace.getConfiguration("earmuff");
  if (!config.get<boolean>("languageServer.enable", true)) {
    return;
  }

  const configured = config.get<string>("languageServer.path", "earmuff-lsp");
  const command = resolveServer(context, configured);
  const serverOptions: ServerOptions = {
    run: { command, transport: TransportKind.stdio },
    debug: { command, transport: TransportKind.stdio },
  };
  const clientOptions: LanguageClientOptions = {
    documentSelector: [{ scheme: "file", language: "earmuff" }],
  };

  client = new LanguageClient(
    "earmuff",
    "earmuff Language Server",
    serverOptions,
    clientOptions
  );

  try {
    await client.start();
    context.subscriptions.push(client);
  } catch (err) {
    client = undefined;
    vscode.window.showWarningMessage(
      `Could not start the earmuff language server ("${command}"). ` +
        `Install it with \`go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest\`, ` +
        `or set "earmuff.languageServer.path" to the binary location. (${String(err)})`
    );
  }
}

// activeEarFile returns the path of the active .ear document, saving it first,
// or undefined (with a message) if there isn't one.
async function activeEarFile(): Promise<string | undefined> {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.languageId !== "earmuff") {
    vscode.window.showErrorMessage("earmuff: no active .ear file.");
    return undefined;
  }
  if (editor.document.isDirty) {
    await editor.document.save();
  }
  return editor.document.uri.fsPath;
}

function cliPath(): string {
  return vscode.workspace
    .getConfiguration("earmuff")
    .get<string>("cli.path", "earmuff");
}

// run invokes the earmuff CLI with args, streaming output to the channel.
// Resolves with the exit code.
function run(args: string[], cwd: string): Promise<number> {
  const cli = cliPath();
  output!.show(true);
  output!.appendLine(`> ${cli} ${args.join(" ")}`);
  return new Promise((resolve) => {
    const proc = cp.spawn(cli, args, { cwd });
    proc.stdout.on("data", (d) => output!.append(d.toString()));
    proc.stderr.on("data", (d) => output!.append(d.toString()));
    proc.on("error", (err) => {
      output!.appendLine(`failed to launch ${cli}: ${err.message}`);
      vscode.window.showErrorMessage(
        `earmuff: could not run "${cli}". Install it with ` +
          "`go install github.com/poolpOrg/earmuff/cmd/earmuff@latest` " +
          'or set "earmuff.cli.path".'
      );
      resolve(-1);
    });
    proc.on("close", (code) => {
      output!.appendLine(`[exit ${code ?? 0}]`);
      resolve(code ?? 0);
    });
  });
}

async function compileMidi(): Promise<void> {
  const file = await activeEarFile();
  if (!file) {
    return;
  }
  const out = file.replace(/\.ear$/i, "") + ".mid";
  const code = await run(["-quiet", "-out", out, file], path.dirname(file));
  if (code === 0) {
    vscode.window.showInformationMessage(`earmuff: wrote ${path.basename(out)}`);
  } else if (code > 0) {
    vscode.window.showErrorMessage("earmuff: compile failed (see the earmuff output).");
  }
}

async function play(): Promise<void> {
  const file = await activeEarFile();
  if (!file) {
    return;
  }
  // Pass the configured player command through as -player, if any. No -quiet:
  // the CLI plays through an available synth.
  const player = vscode.workspace
    .getConfiguration("earmuff")
    .get<string>("player", "")
    .trim();
  const args = player ? ["-player", player, file] : [file];
  const code = await run(args, path.dirname(file));
  if (code > 0) {
    vscode.window.showErrorMessage("earmuff: playback failed (see the earmuff output).");
  }
}

// resolveServer picks the language-server command in priority order:
//   1. an explicit earmuff.languageServer.path setting (if not the default)
//   2. a binary bundled in the extension for this platform
//   3. "earmuff-lsp" on PATH
function resolveServer(context: vscode.ExtensionContext, configured: string): string {
  if (configured && configured !== "earmuff-lsp") {
    return configured; // user override wins
  }
  const folder = platformFolder();
  if (folder) {
    const exe = process.platform === "win32" ? "earmuff-lsp.exe" : "earmuff-lsp";
    const bundled = context.asAbsolutePath(path.join("server", folder, exe));
    if (fs.existsSync(bundled)) {
      return bundled;
    }
  }
  return "earmuff-lsp"; // fall back to PATH
}

// platformFolder maps the current OS/arch to the bundled-server folder name,
// or "" if this platform has no bundled binary.
function platformFolder(): string {
  const key = `${process.platform}-${process.arch}`;
  switch (key) {
    case "darwin-arm64":
      return "darwin-arm64";
    case "darwin-x64":
      return "darwin-x64";
    case "linux-x64":
      return "linux-x64";
    case "linux-arm64":
      return "linux-arm64";
    case "win32-x64":
      return "win32-x64";
    default:
      return "";
  }
}

export function deactivate(): Thenable<void> | undefined {
  return client ? client.stop() : undefined;
}
