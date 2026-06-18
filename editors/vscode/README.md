# earmuff for VS Code

Language support for the [earmuff](https://github.com/poolpOrg/earmuff) music DSL.

This extension provides:

- Syntax highlighting for `.ear` files (comments, strings, keywords, note/chord
  literals, durations, dynamics, numbers, and operators).
- Language-server features (diagnostics, completion, and more) by connecting to
  an external `earmuff-lsp` binary over stdio.

## Requirements

The language-server features require the `earmuff-lsp` binary. Install it with:

```sh
go install github.com/poolpOrg/earmuff/cmd/earmuff-lsp@latest
```

Make sure the resulting binary is on your `PATH`, or point the extension at it
via the `earmuff.languageServer.path` setting (see below). Syntax highlighting
works without the server.

## Settings

| Setting | Type | Default | Description |
| --- | --- | --- | --- |
| `earmuff.languageServer.path` | string | `earmuff-lsp` | Path to the `earmuff-lsp` binary. A bare name is resolved from `PATH`. |
| `earmuff.languageServer.enable` | boolean | `true` | Enable the earmuff language server. |

If the binary cannot be started, the extension shows a warning explaining how to
install or configure it; syntax highlighting continues to work.

## Development

```sh
npm install
npm run compile
```

Then press `F5` in VS Code to launch an Extension Development Host with the
extension loaded. Open a `.ear` file to try it out. Use `npm run watch` to
recompile on save.
