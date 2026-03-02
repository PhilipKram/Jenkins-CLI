package configure

import (
	"fmt"
	"time"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Test Jenkins connection and authentication",
	RunE:  runTest,
}

func runTest(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	fmt.Printf("Testing connection to %s...\n", cfg.URL)

	client, err := clientutil.NewClient(30*time.Second, 3)
	if err != nil {
		return err
	}

	if err := client.TestConnection(cmd.Context()); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Println("✓ Connection successful")
	return nil
}
