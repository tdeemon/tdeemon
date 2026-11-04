# Contributing

Thanks for your interest. tdeemon is in early development: the design and
the plans still move, so open an issue to talk about anything bigger than
a small fix before you start on it.

Questions, and ideas still to talk over, go to
[Discussions](https://github.com/tdeemon/tdeemon/discussions); an issue
is a bug to fix or a feature to build.

Report a security vulnerability privately, as [SECURITY.md](SECURITY.md)
describes, never in a public issue.

Everyone taking part follows the [code of conduct](CODE_OF_CONDUCT.md),
the Contributor Covenant.

## Licence of contributions

tdeemon is under the [AGPL-3.0-only](LICENSE), and so is every
contribution: by opening a pull request you release your change under the
same licence and confirm you have the right to do so, as the
[GitHub Terms of Service](https://docs.github.com/en/site-policy/github-terms/github-terms-of-service#6-contributions-under-repository-license)
also say. There is no Contributor License Agreement to sign.

## Development

The toolchain and every linter are pinned in `mise.toml`; install them
with [mise](https://mise.jdx.dev). `make check` runs what CI checks,
except a few checks that only CI runs, and the git hooks run the fast part
on each commit. The rules for code and tests, and the CI-only checks, are
in [docs/conventions.md](docs/conventions.md).

```sh
mise install        # Go, Node and every linter
lefthook install    # git hooks, once per clone
make check          # make fmt formats, make help lists the targets

TDEEMON_BASE_URL=http://localhost:8080 \
TDEEMON_SECRET_KEY="$(openssl rand -base64 32)" \
TDEEMON_LOG_FORMAT=text \
go run ./cmd/tdeemon
```

Without mise activated in the shell, run the tools as
`mise exec -- <tool>`, such as `mise exec -- lefthook install`. Coding
agents read [AGENTS.md](AGENTS.md).

### Web app

The web app in `web/` (React, TypeScript, Vite) is compiled into the
binary only with the `webui` build tag, so Go builds and tests never need
Node; a binary built without the tag serves a short notice at `/`. To work
on the app, run the server as above and the Vite dev server next to it,
which forwards `/api` to the server (`TDEEMON_DEV_SERVER`, default
`http://localhost:8080`):

```sh
cd web && npm ci
npm run dev     # http://localhost:5173
npm test        # Vitest
npm run lint    # oxlint; npm run format for Prettier
```

The end-to-end tests run the app in Chromium against the binary built
with it: `make e2e-browser` once, then `make e2e`. The app's service
worker runs only in a build, not under `npm run dev`; the end-to-end
tests block it but in `e2e/pwa.spec.ts`.

The docs' screenshots in `docs/screenshots` are made, not drawn:
`make screenshots` signs in as a demo user of made-up data
(`scripts/screenshots-server.sh`, `tdeemon user demo`) and saves each
screen of `web/e2e/docs-screenshots.shot.ts`, on a phone and a desktop,
light and dark. Run it after a change of how the app looks, and commit
what changed.

A binary with the app inside:

```sh
(cd web && npm ci && npm run build)   # writes internal/webui/dist
go build -tags webui ./cmd/tdeemon
```

The build also writes a Brotli and a gzip copy of each script and
stylesheet, and the server sends the copy a browser accepts, so an
instance without a compressing proxy sends about a quarter of the bytes.

### API contract

The JSON API under `/api/v1` is described by an OpenAPI 3.1 document that
is generated from the Go handlers in `internal/api` (with
[huma](https://huma.rocks)). It is served at `/api/v1/openapi.json` and
committed as `api/openapi.json`, and the TypeScript types in
`web/src/api/schema.d.ts` are generated from it. `go test` fails when
`api/openapi.json` is stale, and `npm run check:api` in `web/codegen` when
the types are stale or no longer type-check. After changing the API:

```sh
go test ./internal/api -run TestOpenAPISpec -update
cd web/codegen && npm ci && npm run gen:api
```

## Pull requests

- The title is a Conventional Commit, such as `feat: …` or
  `fix(store): …`; it becomes the commit when the pull request is
  squash-merged.
- CI must pass, and the template's Definition of Done must hold.
- Keep personal data and secrets out of the change: the repository is
  public.
