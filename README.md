# gothic.run

The documentation site for [gothic](https://github.com/metruzanca/gothic),
served at <https://gothic.run>.

The site is a small gothic app. It renders the repository `README.md` as the
landing page and every file under `docs/` as a route. A stdlib markdown parser
in `markdown/` turns headings, paragraphs, lists, tables, quotes, and fenced
code into the block model that `app.gth` renders.

## Version pinning

`go.mod` pins one released gothic version. The markdown comes from the tag with
the same name:

- `go list -m github.com/metruzanca/gothic` decides the version.
- `make docs` clones that tag into `~/.cache/gothic-docs` and copies `README.md`
  and `docs/` into `content/`.
- The Dockerfile clones the same tag and ships the files in the image.

The generated `app_gothic.go` and `app_gothic.js` are produced by the compiler
from that pinned release. The footer links back to the matching tag.

## Development

```sh
make docs       # sync README.md and docs/ into content/
make generate   # regenerate app_gothic.go and app_gothic.js
make run        # serve on :8080, or PORT=... make run
make test       # sync docs, then go test ./...
```

`content/` is gitignored. Tests use `testdata/content`, so `go test ./...` works
without a clone. Set `DOCS_DIR` to serve a different directory.

## Routes

| Route | Source |
|---|---|
| `/` | `README.md` |
| `/guide` | `docs/guide/README.md` |
| `/guide/quickstart` | `docs/guide/01-quickstart.md` |
| `/guide/state` | `docs/guide/02-state.md` |
| `/guide/components` | `docs/guide/03-components.md` |
| `/guide/routing` | `docs/guide/04-routing.md` |
| `/guide/browser` | `docs/guide/05-browser.md` |
| `/guide/long-lists` | `docs/guide/06-long-lists.md` |
| `/guide/architecture` | `docs/guide/07-architecture.md` |
| `/guide/server` | `docs/guide/08-server.md` |
| `/syntax` | `docs/SYNTAX.md` |
| `/modules` | `docs/MODULES.md` |
| `/pitch` | `docs/PITCH.md` |
| `/roadmap` | `docs/PLAN.md` |
| `/releasing` | `docs/RELEASING.md` |

Unknown paths render a not found page with a 404 status.

Repo-relative links to known docs become site routes. Other relative links point
at the matching gothic tag on GitHub.

## Updating gothic

```sh
go get github.com/metruzanca/gothic@vX.Y.Z
make docs
make generate
make test
```

Commit the bumped `go.mod`, `go.sum`, generated files, and any page manifest
changes together. Add or rename entries in `sources` in `content.go` when the
guide changes.

## Deploy

Railway builds the `Dockerfile` from the repository root. The image contains the
binary and the markdown for the pinned release. The service listens on `PORT`
(8080), serves `/healthz`, and is exposed at <https://gothic.run>.

There is no database, session, or client state. Pages are rendered by the
generated gothic view and then returned as static HTML: `staticHTML` removes the
hydration payload, the generated module script, and gothic's bookkeeping
markers. The only browser behavior is the theme toggle, code copy buttons, and
the mobile navigation drawer, all from the small delegated script in `main.go`.

The generated `app_gothic.js` still exists because the gothic compiler always
writes it, but the site never loads it.
