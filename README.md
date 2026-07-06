<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-addressable/brand/main/social/go-ruby-addressable-addressable.png" alt="go-ruby-addressable/addressable" width="720"></p>

# addressable — go-ruby-addressable

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-addressable.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's
[`addressable`](https://github.com/sporkmonger/addressable) gem** — RFC 3986 URI
parsing / normalization / reference-resolution and RFC 6570 URI Templates (all four
levels), byte-exact to the gem. It is a drop-in URI backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a **standalone,
reusable** module with no dependency on the Ruby runtime — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) and
[go-ruby-public-suffix](https://github.com/go-ruby-public-suffix/public-suffix).

## Features

Faithful port of the two headline classes, validated against the `addressable`
gem on every supported platform:

### `Addressable::URI`

- **Parse** any URI reference into `scheme` / `userinfo` / `host` / `port` /
  `path` / `query` / `fragment`, plus reconstructed `authority`.
- **Normalize** per RFC 3986 §6: lowercase scheme + host, IDN → Punycode
  (`normalized_host`), percent-encoding normalization (decode unreserved,
  uppercase escapes), dot-segment removal, default-port stripping.
- **Reference resolution** (`Join` / `+`) per RFC 3986 §5.2 — passes the full
  §5.4 normal + abnormal example set.
- **`query_values`** (ordered pairs *and* last-wins hash) and the assignment side.
- **`omit`**, **`origin`** (RFC 6454).

### `Addressable::Template` (RFC 6570, levels 1-4)

- **`expand(vars)`** for every operator — `{var}` `{+var}` `{#var}` `{.var}`
  `{/var}` `{;var}` `{?var}` `{&var}` — with the `:n` prefix and `*` explode
  modifiers, over string / list / associative-array values.
- **`extract(uri)`** — reverse-match a URI back to its variable bindings.

```go
import "github.com/go-ruby-addressable/addressable"

u := addressable.Parse("HTTP://Example.COM:80/a/./b/../c/")
u.Normalize().String() // "http://example.com/a/c/"

t := addressable.NewTemplate("http://example.com/search{?q,lang}")
t.Expand(map[string]addressable.Value{"q": "hello", "lang": "en"})
// "http://example.com/search?q=hello&lang=en"
t.Extract("http://example.com/search?q=hello&lang=en")
// map[q:hello lang:en]
```

## Tests & coverage

The test suite is **100% coverage** and holds that gate **with no Ruby present** —
the deterministic golden vectors (drawn from the RFC 3986 §5.4 and RFC 6570 example
sets, captured byte-for-byte from the gem) drive every branch. A differential
oracle additionally runs the `addressable` gem where available (gated on
`RUBY_VERSION >= "4.0"`), confirming continued parity. CGO=0; validated on all six
64-bit Go targets (amd64 / arm64 / riscv64 / loong64 / ppc64le / s390x) and three
operating systems.

```
GOWORK=off go test -race -cover ./...
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright © 2026, the
go-ruby-addressable/addressable authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
