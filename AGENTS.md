# Repository Guidelines

## Project Structure & Module Organization

This repository is currently a clean self-hosting workspace; no application modules, test suites, or build configuration are present yet. Keep future additions organized and discoverable:

- `src/` for application or service source code.
- `tests/` for automated tests, mirroring the relevant `src/` paths.
- `config/` for version-controlled, non-secret configuration templates.
- `docs/` for operational notes and architecture documentation.
- `assets/` for static, non-code resources.

Avoid placing generated files, local data volumes, credentials, or machine-specific configuration under version control.

## Build, Test, and Development Commands

There are no established build, test, lint, or development commands at this time. When introducing tooling, expose the normal workflow through a documented, repeatable entry point such as `Makefile`, `package.json`, or a task runner. Document commands in the project README, for example:

```sh
make test    # run the automated test suite
make lint    # check formatting and static analysis
make dev     # start the local development environment
```

Do not add commands that require secrets without documenting the expected environment variables and providing a safe example file.

## Coding Style & Naming Conventions

Follow the formatter and linter native to each language added to the repository, and commit their configuration alongside the code. Use spaces rather than tabs unless the adopted language formatter requires otherwise. Prefer clear, lowercase, hyphen-separated directory names (for example, `config/reverse-proxy/`); use the language's conventional identifier style within code.

Keep configuration files small, commented only where intent is non-obvious, and avoid duplicating values across environments.

## Testing Guidelines

Add tests with behavior changes and bug fixes. Name tests after the behavior they verify, such as `test_rejects_missing_token` or `rejectsMissingToken`. Run the project's complete test command before opening a pull request. Add coverage thresholds only once a test framework and baseline are established.

## Commit & Pull Request Guidelines

No Git history is available yet, so no repository-specific commit convention exists. Use concise imperative commit subjects, such as `Add reverse proxy health check`. Keep unrelated changes separate.

Pull requests should explain the change, note configuration or deployment effects, link relevant issues, and include screenshots or logs when they clarify user-visible or operational behavior. Never include secrets, private keys, or real production configuration in commits or reviews.
