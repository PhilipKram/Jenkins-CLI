package auth

import (
	"fmt"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear stored authentication credentials",
	Long:  `Remove all stored tokens and credentials from the config file. The Jenkins URL and other non-auth settings are preserved.`,
	RunE:  runLogout,
}

func runLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cfg.AuthType = ""
	cfg.User = ""
	cfg.Token = ""
	cfg.BearerToken = ""
	cfg.OAuth = nil

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Logged out. All stored credentials have been cleared.")
	return nil
}
