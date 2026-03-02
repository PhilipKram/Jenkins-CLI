package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token [bearer-token]",
	Short: "Set a bearer token for authentication",
	Long: `Configure the CLI to use a pre-existing bearer/OAuth token for
authentication. The token is sent as an Authorization: Bearer header.

This is useful for CI/CD environments, SSO providers, or when you
already have a token from an external OAuth flow.

Provide the token as an argument or omit it to be prompted interactively.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runToken,
}

func runToken(cmd *cobra.Command, args []string) error {
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

	var bearerToken string
	if len(args) > 0 {
		bearerToken = args[0]
	} else {
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Bearer token: ")
		input, _ := reader.ReadString('\n')
		bearerToken = strings.TrimSpace(input)
	}

	if bearerToken == "" {
		return fmt.Errorf("bearer token cannot be empty")
	}

	existing.AuthType = config.AuthBearer
	existing.BearerToken = bearerToken
	// Clear other auth fields
	existing.User = ""
	existing.Token = ""
	existing.OAuth = nil

	if err := config.Save(existing); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	fmt.Println("Bearer token saved.")
	return nil
}
