#!/usr/bin/env node
"use strict";

/**
 * Thin wrapper that execs the jenkins-cli Go binary with "mcp serve".
 * Used as the npx entry point for MCP server mode.
 */

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");

const BIN_NAME = process.platform === "win32" ? "jenkins-cli.exe" : "jenkins-cli";
const BIN_PATH = path.join(__dirname, BIN_NAME);

if (!fs.existsSync(BIN_PATH)) {
  console.error("jenkins-cli binary not found. Run 'npm install' to download it.");
  process.exit(1);
}

try {
  execFileSync(BIN_PATH, ["mcp", "serve"], {
    stdio: "inherit",
    env: process.env,
  });
} catch (err) {
  if (err.status != null) {
    process.exit(err.status);
  }
  throw err;
}
