# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Go CLI tool for managing Jenkins servers. Built with Cobra framework, supports multiple auth methods (Basic, Bearer, OAuth2+PKCE). Features include real-time build log streaming, build watching with desktop notifications, artifact download, pipeline replay, credential management, view management, and self-upgrade.

## Build & Test Commands

```bash
go build -o jenkins-cli .          # Build binary
go test ./...                      # Run all tests
go test ./internal/jenkins/ -v     # Run tests for a specific package
go test ./internal/jenkins/ -run TestGetJobs  # Run a single test
go vet ./...                       # Static analysis
```

Version is injected at build time via `-ldflags "-X github.com/PhilipKram/jenkins-cli/internal/version.Version=..."`.

## Architecture

**Entry point**: `main.go` → `cmd.Execute()`

**`cmd/`** — Cobra command definitions, one subdirectory per resource group (auth, jobs, builds, nodes, queue, pipeline, plugins, system, credentials, view, upgrade, configure, open). Each package registers subcommands and creates a Jenkins client via a local `newClient()` helper.

**`internal/jenkins/`** — Core API client (`Client` struct). Handles HTTP requests with CSRF crumb support, exponential backoff retries, and context-aware cancellation. Auth is pluggable via the `AuthMethod` interface (BasicAuth, BearerTokenAuth). Includes build log streaming (`stream.go`), pipeline stages (`stages.go`), views (`views.go`), and credentials (`credentials.go`).

**`internal/config/`** — Persists config to `~/.jenkins-cli/config.json` (0600 perms). Supports env var overrides: `JENKINS_URL`, `JENKINS_USER`, `JENKINS_TOKEN`, `JENKINS_BEARER_TOKEN`, `JENKINS_INSECURE`.

**`internal/clientutil/`** — Factory that builds a `jenkins.Client` from config, dispatching on auth type.

**`internal/errors/`** — Structured error types (ConnectionError, AuthenticationError, PermissionError, NotFoundError) with user-facing suggestions.

**`internal/output/`** — Table (tabwriter) and JSON formatters. ANSI status colors for build states.

**`internal/oauth/`** — OAuth2 Authorization Code flow with PKCE.

**`internal/buildwatch/`** — Build monitoring with progress display, polling until completion. Returns exit codes matching Jenkins build status (0=SUCCESS, 1=FAILURE, 2=UNSTABLE, 3=ABORTED).

**`internal/notification/`** — Desktop notifications via `osascript` (macOS) or `notify-send` (Linux). Sends build completion alerts.

**`internal/update/`** — Version checking against GitHub releases, self-update for direct installations, Homebrew detection.

**`internal/version/`** — Version variable set at build time via ldflags.

## Conventions

- Global flags on root command: `--json`, `--timeout` (default 30s), `--retries` (default 3)
- Tests use `httptest.NewServer()` for mock HTTP servers; standard `testing` package only
- All API methods accept `context.Context` as first parameter
- Commands follow pattern: `jenkins-cli <resource> <action> [args] [flags]`
- Build number aliases supported: `last`, `lastSuccessful`
- Build log streaming uses buffered I/O (32KB buffer) to reduce syscall overhead
- URL validation on `open` command to prevent command injection

## Release

Managed via GoReleaser (`.goreleaser.yml`). Push a `v*` tag to trigger GitHub Actions release workflow. Builds for linux/darwin/windows on amd64+arm64.
