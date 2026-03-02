package pipeline

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate [file]",
	Short: "Validate a Jenkinsfile",
	Long: `Validate a Jenkinsfile via the Jenkins pipeline model converter API.

Reads the Jenkinsfile from a file argument or stdin if no file is provided.`,
	Example: `  jenkins-cli pipeline validate Jenkinsfile
  jenkins-cli pipeline validate path/to/Jenkinsfile
  cat Jenkinsfile | jenkins-cli pipeline validate`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var content string

		if len(args) > 0 {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading file: %w", err)
			}
			content = string(data)
		} else {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				content = string(data)
			} else {
				return fmt.Errorf("provide a Jenkinsfile as argument or pipe via stdin")
			}
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		result, err := client.ValidatePipeline(ctx, content)
		if err != nil {
			return err
		}

		if result == "" || strings.Contains(result, "successfully validated") {
			fmt.Println("Jenkinsfile successfully validated.")
			return nil
		}

		fmt.Fprintln(os.Stderr, result)
		os.Exit(1)
		return nil
	},
}

func init() {
	Cmd.AddCommand(validateCmd)
}
