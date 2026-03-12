#!/usr/bin/env node
"use strict";

/**
 * Postinstall script: downloads the correct jenkins-cli binary
 * for the current platform from GitHub Releases.
 */

const fs = require("fs");
const path = require("path");
const https = require("https");
const { execSync } = require("child_process");
const os = require("os");

const REPO = "PhilipKram/Jenkins-CLI";
const BIN_NAME = process.platform === "win32" ? "jenkins-cli.exe" : "jenkins-cli";
const BIN_DIR = path.join(__dirname, "bin");
const BIN_PATH = path.join(BIN_DIR, BIN_NAME);

function getPlatform() {
  const platform = process.platform;
  const arch = process.arch;

  const osMap = { darwin: "darwin", linux: "linux", win32: "windows" };
  const archMap = { x64: "amd64", arm64: "arm64" };

  const goos = osMap[platform];
  const goarch = archMap[arch];

  if (!goos || !goarch) {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }

  return { goos, goarch };
}

function getVersion() {
  const pkg = require("./package.json");
  return pkg.version;
}

function httpsDownload(url, destPath) {
  return new Promise((resolve, reject) => {
    https
      .get(url, { headers: { "User-Agent": "jenkins-cli-mcp-npm" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          return httpsDownload(res.headers.location, destPath).then(resolve, reject);
        }
        if (res.statusCode !== 200) {
          return reject(new Error(`HTTP ${res.statusCode} for ${url}`));
        }
        const file = fs.createWriteStream(destPath);
        res.pipe(file);
        file.on("finish", () => file.close(resolve));
        file.on("error", (err) => {
          fs.unlinkSync(destPath);
          reject(err);
        });
        res.on("error", (err) => {
          fs.unlinkSync(destPath);
          reject(err);
        });
      })
      .on("error", reject);
  });
}

async function main() {
  // Skip download if binary already exists (e.g., CI caching)
  if (fs.existsSync(BIN_PATH)) {
    console.log(`jenkins-cli already exists at ${BIN_PATH}, skipping download`);
    return;
  }

  const { goos, goarch } = getPlatform();
  const version = getVersion();
  const ext = goos === "windows" ? "zip" : "tar.gz";
  const archive = `jenkins-cli_${version}_${goos}_${goarch}.${ext}`;
  const url = `https://github.com/${REPO}/releases/download/v${version}/${archive}`;

  console.log(`Downloading jenkins-cli v${version} for ${goos}/${goarch}...`);
  console.log(`  ${url}`);

  // Ensure bin directory exists
  fs.mkdirSync(BIN_DIR, { recursive: true });

  // Stream download directly to file instead of buffering in memory
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "jenkins-cli-"));
  const archivePath = path.join(tmpDir, archive);
  await httpsDownload(url, archivePath);

  try {
    if (ext === "tar.gz") {
      execSync(`tar -xzf "${archivePath}" -C "${tmpDir}"`, { stdio: "pipe" });
    } else {
      // Windows zip extraction
      execSync(
        `powershell -Command "Expand-Archive -Path '${archivePath}' -DestinationPath '${tmpDir}'"`,
        { stdio: "pipe" }
      );
    }

    // Find the binary in extracted files
    const extracted = path.join(tmpDir, BIN_NAME);
    if (!fs.existsSync(extracted)) {
      throw new Error(`Binary not found in archive at ${extracted}`);
    }

    fs.copyFileSync(extracted, BIN_PATH);
    fs.chmodSync(BIN_PATH, 0o755);

    console.log(`Installed jenkins-cli to ${BIN_PATH}`);
  } finally {
    // Clean up temp directory
    fs.rmSync(tmpDir, { recursive: true, force: true });
  }
}

main().catch((err) => {
  console.error(`Failed to install jenkins-cli: ${err.message}`);
  console.error("");
  console.error("You can install jenkins-cli manually:");
  console.error("  brew install PhilipKram/tap/jenkins-cli");
  console.error("  # or download from https://github.com/PhilipKram/Jenkins-CLI/releases");
  process.exit(1);
});
