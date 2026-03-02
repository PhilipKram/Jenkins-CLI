package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/spf13/cobra"
)

// NewClient creates a Jenkins client based on the current configuration.
// It reads timeout and retries from the root command flags.
func NewClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

// ConfirmPrompt displays a yes/no confirmation prompt and returns the user's choice.
// Returns true if the user confirms (y/yes), false otherwise (n/no or empty).
func ConfirmPrompt(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", message)

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	return input == "y" || input == "yes"
}
