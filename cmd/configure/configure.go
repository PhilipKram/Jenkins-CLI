package configure

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:     "configure",
	Aliases: []string{"config"},
	Short:   "Configure Jenkins connection settings",
	Long: `Set up or update the Jenkins server URL, username, and API token.

Use --profile to create or update a named profile for managing multiple Jenkins servers:
  jenkins-cli configure --profile images --url https://jenkins.example.com/images
  jenkins-cli configure --profile helm --url https://jenkins.example.com/helm`,
	RunE: runConfigure,
}

var (
	flagURL      string
	flagUser     string
	flagToken    string
	flagInsecure bool
)

func init() {
	Cmd.Flags().StringVar(&flagURL, "url", "", "Jenkins server URL")
	Cmd.Flags().StringVar(&flagUser, "user", "", "Jenkins username")
	Cmd.Flags().StringVar(&flagToken, "token", "", "Jenkins API token")
	Cmd.Flags().BoolVar(&flagInsecure, "insecure", false, "Skip TLS certificate verification")

	Cmd.AddCommand(testCmd)
}

func runConfigure(cmd *cobra.Command, args []string) error {
	profileName, _ := cmd.Root().Flags().GetString("profile")

	// Try loading existing config as defaults
	existing, _ := config.Load()
	if existing == nil {
		existing = &config.Config{}
	}

	cfg := &config.Config{
		URL:      flagURL,
		User:     flagUser,
		Token:    flagToken,
		Insecure: flagInsecure,
	}

	reader := bufio.NewReader(os.Stdin)

	if cfg.URL == "" {
		cfg.URL = prompt(reader, "Jenkins URL", existing.URL)
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/")

	if cfg.User == "" {
		cfg.User = prompt(reader, "Username", existing.User)
	}

	if cfg.Token == "" {
		cfg.Token = prompt(reader, "API Token", existing.Token)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("saving config: %w", err)
	}

	if cfg.Insecure {
		fmt.Fprintln(os.Stderr, "WARNING: TLS certificate verification is disabled. Your credentials are vulnerable to man-in-the-middle attacks.")
		fmt.Fprintln(os.Stderr, "This setting persists across all commands. Use proper certificates in production environments.")
	}

	path, _ := config.Path()
	if profileName != "" {
		fmt.Printf("Configuration saved to %s (profile: %s)\n", path, profileName)
	} else {
		effectiveName, _ := config.CurrentProfileName()
		fmt.Printf("Configuration saved to %s (profile: %s)\n", path, effectiveName)
	}
	return nil
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, mask(defaultVal))
	} else {
		fmt.Printf("%s: ", label)
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "" {
		return defaultVal
	}
	return input
}

func mask(s string) string {
	if len(s) <= 4 {
		return "****"
	}
	return s[:4] + strings.Repeat("*", len(s)-4)
}
