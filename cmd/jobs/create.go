package jobs

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var createCmd = &cobra.Command{
	Use:   "create <job-name>",
	Short: "Create a new job from XML configuration",
	Example: `  jenkins-cli jobs create my-new-job -c config.xml`,
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		jobName := args[0]

		configFile, _ := cmd.Flags().GetString("config")
		if configFile == "" {
			return fmt.Errorf("--config flag is required")
		}

		data, err := os.ReadFile(configFile)
		if err != nil {
			return fmt.Errorf("reading config file: %w", err)
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		if err := client.CreateJob(ctx, jobName, string(data)); err != nil {
			return err
		}

		fmt.Printf("Job %q created\n", jobName)
		return nil
	},
}

func init() {
	createCmd.Flags().StringP("config", "c", "", "Path to XML config file (required)")

	Cmd.AddCommand(createCmd)
}
