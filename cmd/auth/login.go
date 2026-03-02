package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/PhilipKram/jenkins-cli/internal/oauth"
	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Jenkins via OAuth2",
	Long: `Perform an OAuth2 Authorization Code flow with PKCE to authenticate
with a Jenkins server that has an OAuth2 plugin configured (e.g. GitHub OAuth,
GitLab OAuth, OpenID Connect).

The command opens your browser, completes the authorization, and stores
the resulting tokens in the CLI config file.`,
	RunE: runLogin,
}

var (
	flagClientID     string
	flagClientSecret string
	flagAuthURL      string
	flagTokenURL     string
	flagScopes       []string
	flagNoBrowser    bool
)

func init() {
	loginCmd.Flags().StringVar(&flagClientID, "client-id", "", "OAuth2 client ID (required)")
	loginCmd.Flags().StringVar(&flagClientSecret, "client-secret", "", "OAuth2 client secret (optional for public clients)")
	loginCmd.Flags().StringVar(&flagAuthURL, "auth-url", "", "OAuth2 authorization endpoint URL (required)")
	loginCmd.Flags().StringVar(&flagTokenURL, "token-url", "", "OAuth2 token endpoint URL (required)")
	loginCmd.Flags().StringSliceVar(&flagScopes, "scope", nil, "OAuth2 scopes to request")
	loginCmd.Flags().BoolVar(&flagNoBrowser, "no-browser", false, "Print URL instead of opening a browser (for headless environments)")

	loginCmd.MarkFlagRequired("client-id")
	loginCmd.MarkFlagRequired("auth-url")
	loginCmd.MarkFlagRequired("token-url")
}

func runLogin(cmd *cobra.Command, args []string) error {
	// Load existing config for Jenkins URL, or create new
	existing, _ := config.Load()
	if existing == nil {
		existing = &config.Config{}
	}

	if existing.URL == "" {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Jenkins URL: ")
		input, _ := reader.ReadString('\n')
		existing.URL = strings.TrimSpace(strings.TrimRight(input, "\n"))
	}

	if existing.URL == "" {
		return fmt.Errorf("Jenkins URL is required. Run 'jenkins-cli configure' or provide via JENKINS_URL")
	}

	oauthCfg := oauth.BuildOAuth2Config(flagClientID, flagClientSecret, flagAuthURL, flagTokenURL, flagScopes)

	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Minute)
	defer cancel()

	var result *oauth.FlowResult
	var err error

	if flagNoBrowser {
		result, err = oauth.ManualFlow(ctx, oauthCfg, func() (string, error) {
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Paste authorization code: ")
			code, err := reader.ReadString('\n')
			return strings.TrimSpace(code), err
		})
	} else {
		result, err = oauth.BrowserFlow(ctx, oauthCfg)
	}

	if err != nil {
		return fmt.Errorf("OAuth login failed: %w", err)
	}

	// Save tokens to config
	existing.AuthType = config.AuthOAuth
	existing.OAuth = &config.OAuthConfig{
		ClientID:     flagClientID,
		ClientSecret: flagClientSecret,
		AuthURL:      flagAuthURL,
		TokenURL:     flagTokenURL,
		Scopes:       flagScopes,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
	}
	// Clear basic auth fields so they don't conflict
	existing.User = ""
	existing.Token = ""
	existing.BearerToken = ""

	if err := config.Save(existing); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Authentication successful! OAuth tokens saved.")
	if result.RefreshToken != "" {
		fmt.Println("A refresh token was saved. Use 'jenkins-cli auth refresh' to renew expired tokens.")
	}
	return nil
}
