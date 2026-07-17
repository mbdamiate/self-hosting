# Gemini CLI Workspace Context - Self-Hosting Workspace

Welcome to the `self-hosting` workspace. This file serves as the foundational context and mandate system for any Gemini CLI session operating within this repository. 

---

## 1. Project Overview & Architecture

* **Purpose:** A clean self-hosting workspace designed for hosting various applications, services, and configuration templates.
* **Current State:** Clean slate. No application modules, test suites, or build configurations are currently present.
* **Organization Mandate:** All future additions must be organized into the following directory structure:
  * `src/` - Application or service source code.
  * `tests/` - Automated tests, directly mirroring the relevant `src/` paths.
  * `config/` - Version-controlled, non-secret configuration templates (e.g., reverse proxy configs).
  * `docs/` - Operational notes, deployment guides, and architecture documentation.
  * `assets/` - Static, non-code resources.
  * **Strict Exclusions:** Never place generated files, local data volumes, secrets/credentials, or machine-specific configurations under version control.

---

## 2. Development, Build, & Test Workflows

No build, test, lint, or development commands are established yet. When tooling is introduced, adhere to these standards:

### Standardized Entry Points
Expose core workflows through documented, repeatable entry points such as a `Makefile`, `package.json`, or a task runner. Example workflow commands should resemble:
* `make test` / `npm test` — Run the automated test suite.
* `make lint` / `npm run lint` — Check formatting and run static analysis tools.
* `make dev` / `npm run dev` — Spin up the local development environment.

### Secrets & Environment Setup
* Do not introduce commands that require secrets without documenting the expected environment variables.
* Provide a safe, non-secret environment template (e.g., `.env.example`) explaining the purpose and format of each variable.

---

## 3. Coding Style & Formatting Conventions

To keep the codebase maintainable and unified across languages:

* **Formatters & Linters:** Adhere strictly to the official formatter and linter native to each programming language introduced. Their configuration files must be committed alongside the source code.
* **Indentation:** Use spaces rather than tabs unless the adopted language's standard formatter dictates tabs (e.g., Go).
* **Directory Naming:** Use clear, lowercase, hyphen-separated directory names (e.g., `config/reverse-proxy/`).
* **Code Identifiers:** Use the conventional style native to the selected programming language (camelCase for TypeScript/JavaScript, snake_case for Python, etc.).
* **Configuration Best Practices:** Keep configuration files lean, commented only when the underlying intent is non-obvious, and avoid duplicate values across different deployment environments.

---

## 4. Testing Guidelines

Tests are a vital component of this workspace.
* **Requirement:** All future code additions, bug fixes, or behavioral changes must include automated tests.
* **Test Location:** Place tests in the `tests/` directory, mirroring the structure of `src/`.
* **Naming Style:** Name tests clearly after the exact behavior they verify (e.g., `test_rejects_missing_token` or `rejectsMissingToken`).
* **Pre-commit Verification:** Always run the complete test suite locally and verify it passes entirely before submitting code or creating a pull request.
* **Coverage:** Establish and enforce coverage thresholds only after the initial test framework and baselines have been fully configured.

---

## 5. Commit & Version Control Practices

* **Commit Style:** Write concise, imperative commit subjects (e.g., `Add reverse proxy health check`). Keep unrelated modifications separated into distinct commits.
* **Pull Request Requirements:**
  * Provide a clear explanation of the change and its impacts.
  * Explicitly note any configuration changes or deployment effects.
  * Link relevant issues or tickets.
  * Attach screenshots or log outputs if they clarify operational or user-visible behaviors.
* **Absolute Security:** Under no circumstances should secrets, API keys, private keys, or actual production credentials be committed to the repository or included in pull requests.
