import * as vscode from "vscode";
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  TransportKind,
} from "vscode-languageclient/node";

let client: LanguageClient | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  const config = vscode.workspace.getConfiguration("earmuff");

  const enabled = config.get<boolean>("languageServer.enable", true);
  if (!enabled) {
    return;
  }

  const command = config.get<string>("languageServer.path", "earmuff-lsp");

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

export function deactivate(): Thenable<void> | undefined {
  if (!client) {
    return undefined;
  }
  return client.stop();
}
