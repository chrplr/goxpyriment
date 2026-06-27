# Viewing the documentation locally

There are two complementary sets of documentation, and you can serve either of
them on your own machine:

- **The API / source-code reference** — generated from the Go source by
  [pkgsite](https://github.com/golang/pkgsite).
- **This documentation site** — the hand-written guides you are reading now,
  built with [zensical](https://zensical.org/).

## API reference (pkgsite)

`pkgsite` serves the Go source documentation (every package, type, and
function) as a local website.

Install it once:

```bash
go install golang.org/x/pkgsite/cmd/pkgsite@latest
```

Then run it from the repository root and open
<http://127.0.0.1:8080> in your browser:

```bash
cd goxpyriment
pkgsite
```

To jump straight to a package, append its import path, for example the
`stimuli` package:

```
http://localhost:8080/github.com/chrplr/goxpyriment@v0.0.0/stimuli
```

## This documentation site (zensical)

The guides on this site are built with zensical. Install it once (it is listed
in `docs/requirements.txt`):

```bash
pip install -r docs/requirements.txt
```

(or follow the [zensical get-started guide](https://zensical.org/docs/get-started/)).

Then, **from the repository root**, start a live-reload preview at
<http://127.0.0.1:8000>:

```bash
zensical serve        # or: make serve
```

To produce a static HTML build in `site/` instead:

```bash
zensical build --clean   # or: make docs
```

!!! note
    Run these commands from the repository root. zensical auto-discovers its
    configuration file (`zensical.toml`), which lives at the root next to the
    `Makefile`.
