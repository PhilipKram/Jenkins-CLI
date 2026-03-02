package update

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Upgrade downloads the given version and replaces the current binary.
func Upgrade(targetVersion string) error {
	url := DownloadURL(targetVersion)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Write to a temp file first.
	tmpArchive, err := os.CreateTemp("", "jenkins-cli-archive-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpArchive.Name())

	if _, err := io.Copy(tmpArchive, resp.Body); err != nil {
		tmpArchive.Close()
		return fmt.Errorf("download failed: %w", err)
	}
	tmpArchive.Close()

	// Extract the binary to a temp directory.
	tmpDir, err := os.MkdirTemp("", "jenkins-cli-upgrade-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	binaryName := "jenkins-cli"
	if runtime.GOOS == "windows" {
		binaryName = "jenkins-cli.exe"
	}

	if runtime.GOOS == "windows" {
		err = extractZip(tmpArchive.Name(), tmpDir, binaryName)
	} else {
		err = extractTarGz(tmpArchive.Name(), tmpDir, binaryName)
	}
	if err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	newBinary := filepath.Join(tmpDir, binaryName)

	// Find the current executable path.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot locate current binary: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("cannot resolve binary path: %w", err)
	}

	// Back up the old binary.
	oldPath := exe + ".old"
	if err := os.Rename(exe, oldPath); err != nil {
		return fmt.Errorf("failed to back up current binary: %w", err)
	}

	// Copy new binary into place.
	if err := copyFile(newBinary, exe); err != nil {
		// Rollback: restore the old binary.
		_ = os.Rename(oldPath, exe)
		return fmt.Errorf("failed to install new binary: %w", err)
	}

	// Clean up the backup.
	_ = os.Remove(oldPath)

	return nil
}

func extractTarGz(archivePath, destDir, targetName string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if filepath.Base(hdr.Name) == targetName && hdr.Typeflag == tar.TypeReg {
			out, err := os.OpenFile(filepath.Join(destDir, targetName), os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
}

func extractZip(archivePath, destDir, targetName string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if filepath.Base(f.Name) == targetName {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out, err := os.OpenFile(filepath.Join(destDir, targetName), os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, rc); err != nil {
				out.Close()
				return err
			}
			out.Close()
			return nil
		}
	}
	return fmt.Errorf("binary %q not found in archive", targetName)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
