package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/PhilipKram/jenkins-cli/internal/oauth"
	"github.com/spf13/cobra"
)

var refreshCmd = &cobra.Command{
	Use:   "refresh",
	Short: "Refresh an expired OAuth access token",
	Long:  `Use the stored refresh token to obtain a new access token from the OAuth provider.`,
	RunE:  runRefresh,
}

func runRefresh(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if cfg.OAuth == nil || cfg.OAuth.RefreshToken == "" {
		return fmt.Errorf("no refresh token found. Run 'jenkins-cli auth login' first")
	}

	oauthCfg := oauth.BuildOAuth2Config(
		cfg.OAuth.ClientID,
		cfg.OAuth.ClientSecret,
		cfg.OAuth.AuthURL,
		cfg.OAuth.TokenURL,
		cfg.OAuth.Scopes,
	)

	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	result, err := oauth.RefreshAccessToken(ctx, oauthCfg, cfg.OAuth.RefreshToken)
	if err != nil {
		return fmt.Errorf("refresh failed: %w", err)
	}

	cfg.OAuth.AccessToken = result.AccessToken
	if result.RefreshToken != "" {
		cfg.OAuth.RefreshToken = result.RefreshToken
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Access token refreshed successfully.")
	return nil
}
