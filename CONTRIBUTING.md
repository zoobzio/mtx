# Contributing to mtx

Thank you for considering contributing to mtx.

## Development Setup

1. Clone the repository
2. Install development tools: `make install-tools`
3. Install git hooks: `make install-hooks`

## Development Workflow

Run `make help` to see all available commands.

Before submitting a pull request:

```bash
make check  # Runs lint, tests, and security scan
```

## Pull Request Process

1. Ensure `make check` passes
2. Update documentation if needed
3. Add tests for new functionality
4. Follow existing code style

## Code Style

- Follow standard Go conventions
- Run `make lint-fix` to auto-fix issues
- Ensure all exported types and functions are documented

## Testing

- Maintain 1:1 relationship between source files and test files
- Place test helpers in `testing/` directory
- Coverage target: 70% project, 80% for new code
