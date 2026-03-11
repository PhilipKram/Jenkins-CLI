# Jenkins CLI

A command-line tool for controlling Jenkins from the terminal, built with Go.

## Installation

### Homebrew (macOS & Linux)

```bash
brew install PhilipKram/tap/jenkins-cli
```

### Download Binary

Download a prebuilt binary from the [Releases](https://github.com/PhilipKram/Jenkins-CLI/releases) page.

### Go Install

```bash
go install github.com/PhilipKram/jenkins-cli@latest
```

### Build from Source

```bash
git clone https://github.com/PhilipKram/Jenkins-CLI.git
cd Jenkins-CLI
go build -o jenkins-cli .
```

## Authentication

Jenkins CLI supports three authentication methods: **Basic** (username + API token), **OAuth2** (browser-based login), and **Bearer token** (manual token).

### Basic Authentication (default)

```bash
jenkins-cli configure
```

This will prompt for your Jenkins URL, username, and API token. Generate an API token from Jenkins → Your Name → Configure → API Token.

### OAuth2 Login (browser-based)

For Jenkins servers with an OAuth2 plugin (GitHub OAuth, GitLab OAuth, OpenID Connect):

```bash
# Opens your browser for OAuth2 Authorization Code flow with PKCE
jenkins-cli auth login \
  --client-id YOUR_CLIENT_ID \
  --auth-url https://your-idp.com/authorize \
  --token-url https://your-idp.com/token \
  --scope openid,profile

# For headless/SSH environments — prints URL, prompts for code
jenkins-cli auth login \
  --client-id YOUR_CLIENT_ID \
  --auth-url https://your-idp.com/authorize \
  --token-url https://your-idp.com/token \
  --no-browser

# Refresh an expired OAuth access token
jenkins-cli auth refresh
```

### Bearer Token

For SSO providers, CI/CD pipelines, or when you already have a token:

```bash
# Set a bearer token interactively
jenkins-cli auth token

# Set a bearer token directly
jenkins-cli auth token eyJhbGciOiJSUzI1NiIs...
```

### Auth Management

```bash
# Check current auth method and credentials
jenkins-cli auth status

# Clear all stored credentials
jenkins-cli auth logout
```

### Testing Configuration

Verify your Jenkins connection and authentication:

```bash
# Test connection, authentication, and permissions
jenkins-cli configure test
```

This command validates:
- URL is reachable
- Authentication credentials are valid
- Required permissions are granted

Useful for troubleshooting connection issues before running automation.

### Multiple Jenkins Servers (Profiles)

Manage multiple Jenkins servers with named profiles, each with its own URL and credentials:

```bash
# Create profiles for different Jenkins servers
jenkins-cli configure --profile images --url https://jenkins.example.com/ssbu-01
jenkins-cli configure --profile helm --url https://jenkins.example.com/allotsecure-01/
jenkins-cli configure --profile hotfix --url https://jenkins.example.com/allotsecure-01/job/Lock_and_Release_Hotfix_Image/

# Use a profile for a single command
jenkins-cli --profile images jobs list
jenkins-cli -p helm builds list my-pipeline

# Set the active profile (used when --profile is not specified)
jenkins-cli profile use images

# List all configured profiles
jenkins-cli profile list

# Show profile details
jenkins-cli profile show images

# Delete a profile
jenkins-cli profile delete old-server
```

Profile resolution order:
1. `--profile` / `-p` flag
2. `JENKINS_PROFILE` environment variable
3. `current_profile` setting in config file
4. `"default"` profile

Existing single-server configs are automatically migrated to a `"default"` profile.

#### Multiple MCP Servers

Register separate MCP server instances for each Jenkins profile:

```bash
jenkins-cli mcp install --profile images --name jenkins-images
jenkins-cli mcp install --profile helm --name jenkins-helm
jenkins-cli mcp install --profile hotfix --name jenkins-hotfix
```

Each MCP server instance uses the `JENKINS_PROFILE` environment variable to select its profile.

### Environment Variables

Override config file settings with environment variables:

```bash
export JENKINS_URL=https://jenkins.example.com
export JENKINS_USER=admin
export JENKINS_TOKEN=your-api-token
export JENKINS_BEARER_TOKEN=your-bearer-token  # overrides auth type to bearer
export JENKINS_INSECURE=true                   # skip TLS verification
export JENKINS_PROFILE=images                  # select a profile
```

Configuration is stored in `~/.jenkins-cli/config.json` with `0600` permissions.

## Usage

### Jobs

```bash
# List all jobs
jenkins-cli jobs list

# List jobs in a folder
jenkins-cli jobs list my-folder

# Show job details
jenkins-cli jobs info my-pipeline

# Trigger a build
jenkins-cli jobs build my-pipeline

# Trigger a parameterized build
jenkins-cli jobs build my-pipeline -p BRANCH=main -p ENV=staging

# Trigger and wait for completion (with desktop notification)
jenkins-cli jobs build my-pipeline --wait --follow --timeout 30m

# Enable/disable a job
jenkins-cli jobs enable my-pipeline
jenkins-cli jobs disable my-pipeline

# Delete a job
jenkins-cli jobs delete old-job
```

### Builds

```bash
# List recent builds
jenkins-cli builds list my-pipeline
jenkins-cli builds list my-pipeline -n 20

# Show build details
jenkins-cli builds info my-pipeline 42

# Show last build
jenkins-cli builds last my-pipeline

# View console output
jenkins-cli builds log my-pipeline 42

# Stream logs in real time
jenkins-cli builds log my-pipeline 42 -f

# Stop a running build (graceful)
jenkins-cli builds stop my-pipeline 42

# Force kill a running build
jenkins-cli builds kill my-pipeline 42
jenkins-cli builds kill my-pipeline 42 -y  # skip confirmation

# Watch a build until completion (with desktop notification)
jenkins-cli builds watch my-pipeline 42
jenkins-cli builds watch my-pipeline last --timeout 30m

# List and download build artifacts
jenkins-cli builds artifacts my-pipeline 42
jenkins-cli builds artifacts my-pipeline last --download --output-dir ./out
jenkins-cli builds artifacts my-pipeline 42 --download --filter "*.jar"
```

### Pipeline

```bash
# Replay a pipeline build with the original Jenkinsfile
jenkins-cli pipeline replay my-pipeline 42

# Replay with a modified Jenkinsfile
jenkins-cli pipeline replay my-pipeline 42 -s modified-Jenkinsfile

# Replay and stream logs
jenkins-cli pipeline replay my-pipeline last -f
```

### Credentials

```bash
# List all credentials
jenkins-cli credentials list

# List credentials in a specific domain
jenkins-cli credentials list my-domain

# Show credential details
jenkins-cli credentials info my-cred-id

# Create a new credential (interactive)
jenkins-cli credentials create --type username-password
jenkins-cli credentials create --type secret-text
jenkins-cli credentials create --type ssh-key
jenkins-cli credentials create --type certificate

# Delete a credential
jenkins-cli credentials delete my-cred-id
```

### Views

```bash
# List all views
jenkins-cli view list

# Show view details
jenkins-cli view info my-view

# Create a new view
jenkins-cli view create my-view

# Create a view with custom XML config
jenkins-cli view create my-view -c config.xml

# Add/remove jobs from a view
jenkins-cli view add-job my-view my-pipeline
jenkins-cli view remove-job my-view my-pipeline

# Delete a view
jenkins-cli view delete my-view
jenkins-cli view delete my-view -y  # skip confirmation
```

### Nodes

```bash
# List all nodes/agents
jenkins-cli nodes list

# Show node details
jenkins-cli nodes info my-agent

# Take a node offline
jenkins-cli nodes offline my-agent -m "Maintenance"

# Bring a node online
jenkins-cli nodes online my-agent
```

### Build Queue

```bash
# List queued builds
jenkins-cli queue list

# Cancel a queued item
jenkins-cli queue cancel 123
```

### Plugins

```bash
# List installed plugins
jenkins-cli plugins list

# Show only plugins with updates
jenkins-cli plugins list -u
```

### System

```bash
# Show Jenkins system info and version
jenkins-cli system info

# Show current user
jenkins-cli system whoami

# Safe restart (waits for builds to finish)
jenkins-cli system restart --safe

# Enter/exit quiet mode
jenkins-cli system quiet-down
jenkins-cli system quiet-down --cancel
```

### Open in Browser

```bash
# Open Jenkins dashboard
jenkins-cli open

# Open a specific job
jenkins-cli open my-pipeline
```

### Upgrade

```bash
# Upgrade jenkins-cli to the latest version
jenkins-cli upgrade
```

Automatically detects Homebrew installations and directs you to use `brew upgrade` instead.

### JSON Output

Most commands support `--json` for machine-readable output:

```bash
jenkins-cli jobs list --json
jenkins-cli builds list my-pipeline --json | jq '.[0]'
```

### Global Flags

```
-p, --profile   Configuration profile to use
    --json      Output in JSON format
    --timeout   Request timeout (default: 30s)
    --retries   Maximum number of retries (default: 3)
```

## Shell Completion

```bash
# Bash
jenkins-cli completion bash > /etc/bash_completion.d/jenkins-cli

# Zsh
jenkins-cli completion zsh > "${fpath[1]}/_jenkins-cli"

# Fish
jenkins-cli completion fish > ~/.config/fish/completions/jenkins-cli.fish
```

## Project Structure

```
├── main.go                     # Entry point
├── cmd/
│   ├── root.go                 # Root command setup
│   ├── helpers.go              # Shared CLI helpers
│   ├── auth/                   # Auth commands (login, token, status, logout, refresh)
│   ├── configure/              # Configure & connection test commands
│   ├── profile/                # Profile management commands (list, use, show, delete)
│   ├── jobs/                   # Jobs commands
│   ├── builds/                 # Builds commands (log, watch, artifacts, kill)
│   ├── pipeline/               # Pipeline commands (replay)
│   ├── credentials/            # Credential management commands
│   ├── view/                   # View management commands
│   ├── nodes/                  # Nodes commands
│   ├── queue/                  # Queue commands
│   ├── plugins/                # Plugins commands
│   ├── system/                 # System commands
│   ├── upgrade/                # Self-upgrade command
│   └── open/                   # Open-in-browser command
└── internal/
    ├── jenkins/                # Jenkins API client
    │   ├── client.go           # HTTP client with auth & CSRF crumb support
    │   ├── jobs.go             # Job operations
    │   ├── builds.go           # Build operations
    │   ├── nodes.go            # Node operations
    │   ├── views.go            # View operations
    │   ├── credentials.go      # Credential operations
    │   ├── stages.go           # Pipeline stage operations
    │   ├── stream.go           # Build log streaming
    │   ├── queue.go            # Queue operations
    │   ├── plugins.go          # Plugin operations
    │   └── system.go           # System operations
    ├── buildwatch/             # Build monitoring & progress display
    ├── notification/           # Desktop notifications (macOS/Linux)
    ├── update/                 # Version checking & self-update
    ├── version/                # Version info (set via ldflags)
    ├── config/                 # Configuration management (multi-auth)
    ├── clientutil/             # Auth dispatch (config → client)
    ├── errors/                 # Structured error types with suggestions
    ├── oauth/                  # OAuth2 flow (auth code + PKCE, refresh)
    └── output/                 # Table & JSON output formatting
```

## Releasing

Releases are automated via [GoReleaser](https://goreleaser.com/) and GitHub Actions.

To create a new release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

This will:
1. Run tests
2. Build binaries for linux/darwin/windows (amd64 + arm64)
3. Create a GitHub release with archives and checksums
4. Update the Homebrew formula in [PhilipKram/homebrew-tap](https://github.com/PhilipKram/homebrew-tap)

### Setup for Homebrew Tap

1. Create a repository named `homebrew-tap` under your GitHub account
2. Add a `HOMEBREW_TAP_TOKEN` secret to this repo — a GitHub PAT with `repo` scope that can push to the tap repository

## License

MIT
