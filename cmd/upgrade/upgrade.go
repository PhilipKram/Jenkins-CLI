package upgrade

import (
	"fmt"
	"os"

	"github.com/PhilipKram/jenkins-cli/internal/update"
	"github.com/PhilipKram/jenkins-cli/internal/version"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade jenkins-cli to the latest version",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if version.Version == "dev" {
			return fmt.Errorf("cannot upgrade a development build — install a release version first")
		}

		if update.DetectInstallMethod() == update.InstallHomebrew {
			fmt.Fprintln(os.Stderr, "jenkins-cli was installed via Homebrew. Please upgrade with:")
			fmt.Fprintln(os.Stderr, "  brew upgrade jenkins-cli")
			return nil
		}

		fmt.Fprintf(os.Stderr, "Checking for updates...\n")

		release, err := update.FetchLatestRelease()
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}

		current, err := update.ParseSemver(version.Version)
		if err != nil {
			return fmt.Errorf("invalid current version: %w", err)
		}

		latest, err := update.ParseSemver(release.TagName)
		if err != nil {
			return fmt.Errorf("invalid latest version: %w", err)
		}

		if !latest.IsNewer(current) {
			fmt.Fprintf(os.Stderr, "Already up to date (v%s)\n", current)
			return nil
		}

		fmt.Fprintf(os.Stderr, "Upgrading v%s -> v%s...\n", current, latest)

		if err := update.Upgrade(latest.String()); err != nil {
			return fmt.Errorf("upgrade failed: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Successfully upgraded to v%s\n", latest)
		return nil
	},
}
