# earmuff.io landing page

A self-contained marketing landing page for [earmuff](https://github.com/poolpOrg/earmuff).
No build step — it's a single `index.html` plus `assets/`.

## Preview locally

```sh
python3 -m http.server -d landing 8000   # then open http://localhost:8000
```

## Deployment (earmuff.io)

The intended layout for the published site is:

```
/            ← this landing page (index.html, assets/)
/docs/       ← the Hugo documentation site (built from website/ into docs/)
CNAME        ← earmuff.io
```

GitHub Pages serves one site per repo, so to host both the landing page (apex)
and the docs (under `/docs/`) from this repo, the publish root must contain this
page's files at top level and the Hugo `docs/` output nested beneath. A small
deploy step (or CI) assembles that root: copy `landing/*` to the root and keep
`docs/` as-is, then point Pages + the `earmuff.io` DNS at it.

Until DNS is wired up, the "Docs" links here use the `/docs/` path, which
resolves once the combined site is published.
