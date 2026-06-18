# earmuff.io landing page

A self-contained marketing landing page for [earmuff](https://github.com/poolpOrg/earmuff).
No build step — a single `index.html` plus `assets/`.

## Preview locally

```sh
python3 -m http.server -d landing 8000   # then open http://localhost:8000
```

## Deployment

The whole site is built and deployed to GitHub Pages by
[`.github/workflows/pages.yml`](../.github/workflows/pages.yml) on every push to
`main`. It assembles:

```
/            ← this landing page (index.html, assets/)
/docs/       ← the Hugo docs site (built from website/)
CNAME        ← earmuff.io
```

So `earmuff.io` serves the landing page and `earmuff.io/docs/` serves the docs.
To go live, enable GitHub Pages with the **GitHub Actions** source and point the
`earmuff.io` DNS at GitHub Pages.
