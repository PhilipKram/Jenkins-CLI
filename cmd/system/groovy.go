package system

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var groovyCmd = &cobra.Command{
	Use:   "groovy [file]",
	Short: "Execute a Groovy script on the Jenkins master",
	Long: `Execute a Groovy script on the Jenkins master via the script console.

Reads the script from a file argument, --script flag, or stdin.`,
	Example: `  jenkins-cli system groovy script.groovy
  jenkins-cli system groovy -s 'println Jenkins.instance.numExecutors'
  echo 'println "hello"' | jenkins-cli system groovy`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var script string

		inlineScript, _ := cmd.Flags().GetString("script")
		if len(args) > 0 {
			data, err := os.ReadFile(args[0])
			if err != nil {
				return fmt.Errorf("reading script file: %w", err)
			}
			script = string(data)
		} else if inlineScript != "" {
			script = inlineScript
		} else {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				script = string(data)
			} else {
				return fmt.Errorf("provide a script file, --script flag, or pipe via stdin")
			}
		}

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		ctx := cmd.Context()
		result, err := client.ExecuteGroovy(ctx, script)
		if err != nil {
			return err
		}

		if result != "" {
			fmt.Print(result)
		}
		return nil
	},
}

func init() {
	groovyCmd.Flags().StringP("script", "s", "", "Inline Groovy script")

	Cmd.AddCommand(groovyCmd)
}
