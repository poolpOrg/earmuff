# Publishing the earmuff VS Code extension

The extension publishes to the **VS Code Marketplace** (Microsoft) and
**Open VSX** (used by VSCodium, Cursor, Gitpod, …). Most of it is automated by
`.github/workflows/release.yml`; the steps below are the one-time setup only you
can do, because they involve your accounts and secret tokens.

## One-time setup

### 1. Create the Marketplace publisher
The extension's `publisher` is **`poolpOrg`** (in `package.json`). A publisher
with that exact ID must exist:

1. Sign in at <https://marketplace.visualstudio.com/manage> with a Microsoft
   account.
2. Create a publisher with ID `poolpOrg` (or change `publisher` in
   `package.json` to one you own).

### 2. Get an Azure DevOps token (VSCE_PAT)
The Marketplace is gated by Azure DevOps.

1. Go to <https://dev.azure.com> → User Settings → Personal Access Tokens.
2. Create a token: Organization = **All accessible organizations**,
   Scopes = **Marketplace → Manage**.
3. Copy it; you won't see it again.

### 3. Get an Open VSX token (OVSX_PAT)
1. Sign in at <https://open-vsx.org> (GitHub login).
2. Create a namespace matching the publisher (`poolpOrg`) and an access token
   under your profile's Access Tokens.

### 4. Add the tokens as GitHub repository secrets
In the GitHub repo → Settings → Secrets and variables → Actions, add:
- `VSCE_PAT` — the Azure DevOps token
- `OVSX_PAT` — the Open VSX token

## Releasing

Bump `version` in `editors/vscode/package.json`, commit, then push a matching
tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The Release workflow then cross-compiles the language server, packages the
`.vsix`, publishes to whichever registries have a token configured, and attaches
the `.vsix` to a GitHub Release. (Steps for a missing token are skipped, so the
workflow still builds a `.vsix` and a Release without any secrets.)

## Publishing by hand (alternative)

```sh
cd editors/vscode
npm install
npm run package          # builds server, compiles, makes the .vsix

# Marketplace
npx vsce login poolpOrg  # paste the Azure DevOps PAT once
npx vsce publish --no-dependencies

# Open VSX
npx ovsx publish *.vsix -p <OVSX_PAT>
```

## Before the first publish

- Replace the placeholder icon at `media/icon.svg` (re-rasterize to
  `media/icon.png` with `rsvg-convert -w 128 -h 128 media/icon.svg -o media/icon.png`).
- Review the listing copy in `README.md` (it is shown on the Marketplace page).
