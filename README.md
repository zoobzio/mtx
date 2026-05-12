# mtx

[![CI Status](https://github.com/zoobzio/mtx/workflows/CI/badge.svg)](https://github.com/zoobzio/mtx/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/zoobzio/mtx/graph/badge.svg?branch=main)](https://codecov.io/gh/zoobzio/mtx)
[![Go Report Card](https://goreportcard.com/badge/github.com/zoobzio/mtx)](https://goreportcard.com/report/github.com/zoobzio/mtx)
[![CodeQL](https://github.com/zoobzio/mtx/workflows/CodeQL/badge.svg)](https://github.com/zoobzio/mtx/security/code-scanning)
[![Go Reference](https://pkg.go.dev/badge/github.com/zoobzio/mtx.svg)](https://pkg.go.dev/github.com/zoobzio/mtx)
[![License](https://img.shields.io/github/license/zoobzio/mtx)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zoobzio/mtx)](go.mod)
[![Release](https://img.shields.io/github/v/release/zoobzio/mtx)](https://github.com/zoobzio/mtx/releases)

Template repository for standing up Go library infrastructure matching the zoobzio ecosystem. Clone it, rename it, start building.

## What's Included

```
your-new-package/
├── go.mod                    # Go 1.24+ with toolchain
├── Makefile                  # test, lint, coverage, ci targets
├── .golangci.yml             # v2 config with security linters
├── .codecov.yml              # 70% project / 80% patch targets
├── .goreleaser.yml           # Library release automation
├── .github/workflows/
│   ├── ci.yml                # Test matrix, lint, security, benchmarks
│   ├── coverage.yml          # Codecov integration
│   ├── release.yml           # Tag-triggered releases
│   └── codeql.yml            # Weekly security analysis
├── testing/
│   ├── helpers.go            # Reusable test utilities
│   ├── integration/          # Integration test structure
│   └── benchmarks/           # Benchmark structure
├── LICENSE                   # MIT
├── CONTRIBUTING.md           # Development workflow
└── SECURITY.md               # Vulnerability reporting
```

## Install

This is a GitHub template repository. Click **Use this template** on GitHub, or:

```bash
gh repo create your-package --template zoobzio/mtx --clone
cd your-package
```

Then update `go.mod`, `Makefile`, and `.goreleaser.yml` with your package name.

## Quick Start

After creating your repository from the template:

```bash
# Update module path
sed -i 's/mtx/your-package/g' go.mod Makefile .goreleaser.yml

# Install development tools
make install-tools

# Verify everything works
make check

# Start building
```

## Capabilities

| Feature | Description | Configuration |
|---------|-------------|---------------|
| Multi-version testing | Go 1.24 and 1.25 matrix | `.github/workflows/ci.yml` |
| Linting | golangci-lint v2 with security focus | `.golangci.yml` |
| Coverage tracking | Codecov with PR comments | `.codecov.yml` |
| Release automation | GoReleaser for library packages | `.goreleaser.yml` |
| Security scanning | gosec + CodeQL | `.github/workflows/codeql.yml` |
| Benchmarks | Structured benchmark directory | `testing/benchmarks/` |

## Why mtx?

- **Immediate CI**: Push and your workflows run—no setup required
- **Consistent tooling**: Every zoobzio package uses the same linter config, coverage targets, and release process
- **Security by default**: gosec, CodeQL, and security-focused linters enabled from day one
- **Test infrastructure ready**: Helpers, integration tests, and benchmarks structured and waiting

## The Zoobzio Ecosystem

All zoobzio Go libraries use mtx as their foundation. When standards evolve, updates propagate from here.

## Documentation

- **Learn**: Review the configuration files directly—they're documented inline
- **Guides**: See `CONTRIBUTING.md` for development workflow
- **Reference**: Run `make help` for available commands

## Contributing

Development workflow and conventions are documented in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT License - see [LICENSE](LICENSE)
