package jobs

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config <job-name>",
	Short: "Export or import job XML configuration",
	Long: `Export or import a job's XML configuration.

By default, exports the job config XML to stdout. Use --output to write to a file.
Use --import to update a job's configuration from an XML file.`,
	Example: `  jenkins-cli jobs config my-job
  jenkins-cli jobs config my-job -o config.xml
  jenkins-cli jobs config my-job -i config.xml`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		jobName := args[0]

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		importFile, _ := cmd.Flags().GetString("import")
		if importFile != "" {
			data, err := os.ReadFile(importFile)
			if err != nil {
				return fmt.Errorf("reading config file: %w", err)
			}
			if err := client.UpdateJobConfig(ctx, jobName, string(data)); err != nil {
				return err
			}
			fmt.Printf("Configuration updated for job %q\n", jobName)
			return nil
		}

		// Export
		config, err := client.GetJobConfig(ctx, jobName)
		if err != nil {
			return err
		}

		outputFile, _ := cmd.Flags().GetString("output")
		if outputFile != "" {
			if err := os.WriteFile(outputFile, []byte(config), 0644); err != nil {
				return fmt.Errorf("writing config file: %w", err)
			}
			fmt.Printf("Configuration exported to %s\n", outputFile)
			return nil
		}

		fmt.Print(config)
		return nil
	},
}

func init() {
	configCmd.Flags().StringP("import", "i", "", "Import config XML from file")
	configCmd.Flags().StringP("output", "o", "", "Write exported config to file")

	Cmd.AddCommand(configCmd)
}
