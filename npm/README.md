# @philipkram/jenkins-cli-mcp

MCP (Model Context Protocol) server for [jenkins-cli](https://github.com/PhilipKram/Jenkins-CLI) — manage Jenkins jobs, builds, pipelines, nodes, credentials, and more from AI assistants.

## Quick Start

### Claude Code

```json
{
  "mcpServers": {
    "jenkins-cli": {
      "command": "npx",
      "args": ["-y", "@philipkram/jenkins-cli-mcp"]
    }
  }
}
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "jenkins-cli": {
      "command": "npx",
      "args": ["-y", "@philipkram/jenkins-cli-mcp"],
      "env": {
        "JENKINS_PROFILE": "default"
      }
    }
  }
}
```

## Configuration

No manual setup required. On first use, the AI assistant will automatically detect that Jenkins is not configured and use the built-in `config_set` MCP tool to set up your connection interactively.

Alternatively, set environment variables in the MCP config:

```json
{
  "mcpServers": {
    "jenkins-cli": {
      "command": "npx",
      "args": ["-y", "@philipkram/jenkins-cli-mcp"],
      "env": {
        "JENKINS_URL": "https://jenkins.example.com",
        "JENKINS_USER": "admin",
        "JENKINS_TOKEN": "your-api-token"
      }
    }
  }
}
```

## Available Tools

| Category | Tools |
|----------|-------|
| **Jobs** | `job_list`, `job_view`, `job_build`, `job_enable`, `job_disable` |
| **Builds** | `build_list`, `build_view`, `build_log`, `build_last`, `build_stop`, `build_artifacts` |
| **Pipeline** | `pipeline_validate`, `pipeline_stages`, `pipeline_stage_log` |
| **Nodes** | `node_list`, `node_view` |
| **Queue** | `queue_list`, `queue_cancel` |
| **Credentials** | `credential_list`, `credential_view` |
| **Views** | `view_list` |
| **System** | `system_info` |
| **Multibranch** | `multibranch_branches`, `multibranch_scan`, `multibranch_scan_log` |

## How It Works

This npm package downloads the pre-built `jenkins-cli` Go binary for your platform during `npm install` (from [GitHub Releases](https://github.com/PhilipKram/Jenkins-CLI/releases)), then runs its built-in MCP server over stdio.

## License

MIT
