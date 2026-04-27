# Contributing to Forge

First off, thank you for considering contributing to Forge! It's people like you who make Forge such a great tool for the community.

## Development Workflow

### 1. Prerequisites

- **Go 1.23+**
- **Node.js 22.19.0** (Managed by **Volta**)
- **pnpm 10.32.1** (Managed by **Volta**)
- **Docker** (running locally)
- **golangci-lint**

### 2. Setting Up Your Environment

```bash
git clone https://github.com/0xdaksh-12/Forge.git
cd Forge
make dev
```

The `make dev` command will start the Go backend and the Vite frontend in parallel with live reloading.

### 3. Making Changes

- **Backend**: Most core logic is in `internal/engine` (orchestration) and `internal/api` (HTTP handlers).
- **Frontend**: Located in the `web` directory.
- **Documentation**: If you change API behavior, remember to run `make swagger` to update the docs.

## Coding Standards

### Linting

We enforce strict linting. Before submitting a PR, please run:

```bash
make lint
```

Your code must pass all linters defined in `.golangci.yml`.

### Testing

We value stability. If you add a new feature, please add a corresponding unit test.

```bash
make test
```

### Commit Messages

We follow the **Conventional Commits** (Google-style) specification:

- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code (white-space, formatting, etc.)
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `build`: Changes that affect the build system or external dependencies

## Pull Request Process

1. Fork the repository and create your branch from `main`.
2. Ensure the test suite passes and the linter is happy.
3. Update the documentation if you've changed any public-facing behavior.
4. Submit your PR with a clear description of the problem and your solution.

Thank you for contributing!
