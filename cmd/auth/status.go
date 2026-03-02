package auth

import (
	"fmt"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current authentication status",
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	fmt.Printf("Jenkins URL:  %s\n", cfg.URL)
	fmt.Printf("Auth method:  %s\n", cfg.EffectiveAuthType())

	switch cfg.EffectiveAuthType() {
	case config.AuthBasic:
		if cfg.User != "" {
			fmt.Printf("User:         %s\n", cfg.User)
		}
		if cfg.Token != "" {
			fmt.Printf("API token:    %s\n", maskToken(cfg.Token))
		}
	case config.AuthBearer:
		if cfg.BearerToken != "" {
			fmt.Printf("Bearer token: %s\n", maskToken(cfg.BearerToken))
		}
	case config.AuthOAuth:
		if cfg.OAuth != nil {
			fmt.Printf("Client ID:    %s\n", cfg.OAuth.ClientID)
			fmt.Printf("Auth URL:     %s\n", cfg.OAuth.AuthURL)
			fmt.Printf("Token URL:    %s\n", cfg.OAuth.TokenURL)
			if cfg.OAuth.AccessToken != "" {
				fmt.Printf("Access token: %s\n", maskToken(cfg.OAuth.AccessToken))
			}
			if cfg.OAuth.RefreshToken != "" {
				fmt.Println("Refresh token: present")
			}
			if len(cfg.OAuth.Scopes) > 0 {
				fmt.Printf("Scopes:       %v\n", cfg.OAuth.Scopes)
			}
		}
	}

	return nil
}

func maskToken(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "..." + s[len(s)-4:]
}
