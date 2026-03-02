package system

import (
	"fmt"
	"os"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/PhilipKram/jenkins-cli/internal/output"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

var Cmd = &cobra.Command{
	Use:   "system",
	Short: "Jenkins system operations",
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show Jenkins system information",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		info, err := client.GetSystemInfo(ctx)
		if err != nil {
			return err
		}

		version, _ := client.GetVersion(ctx)

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			data := map[string]any{
				"version":         version,
				"mode":            info.Mode,
				"nodeDescription": info.NodeDescription,
				"numExecutors":    info.NumExecutors,
				"useSecurity":     info.UseSecurity,
				"quietingDown":    info.QuietingDown,
			}
			return output.PrintJSON(os.Stdout, data)
		}

		fmt.Printf("Version:     %s\n", version)
		fmt.Printf("Mode:        %s\n", info.Mode)
		fmt.Printf("Description: %s\n", info.NodeDescription)
		fmt.Printf("Executors:   %d\n", info.NumExecutors)
		fmt.Printf("Security:    %v\n", info.UseSecurity)
		fmt.Printf("Quieting:    %v\n", info.QuietingDown)
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show current authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		who, err := client.WhoAmI(ctx)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, who)
		}

		fmt.Printf("User: %s\n", who.Name)
		if len(who.Authorities) > 0 {
			fmt.Println("Roles:")
			for _, a := range who.Authorities {
				fmt.Printf("  - %s\n", a)
			}
		}
		return nil
	},
}

var restartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart Jenkins",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		safe, _ := cmd.Flags().GetBool("safe")
		if safe {
			if err := client.SafeRestart(ctx); err != nil {
				return err
			}
			fmt.Println("Safe restart initiated (will wait for running builds to finish)")
		} else {
			if err := client.Restart(ctx); err != nil {
				return err
			}
			fmt.Println("Restart initiated")
		}
		return nil
	},
}

var quietCmd = &cobra.Command{
	Use:   "quiet-down",
	Short: "Put Jenkins into quiet mode (no new builds)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		cancel, _ := cmd.Flags().GetBool("cancel")
		if cancel {
			if err := client.CancelQuietDown(ctx); err != nil {
				return err
			}
			fmt.Println("Quiet mode cancelled")
		} else {
			if err := client.QuietDown(ctx); err != nil {
				return err
			}
			fmt.Println("Jenkins entering quiet mode")
		}
		return nil
	},
}

func init() {
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	whoamiCmd.Flags().Bool("json", false, "Output in JSON format")
	restartCmd.Flags().Bool("safe", false, "Safe restart (wait for builds to finish)")
	quietCmd.Flags().Bool("cancel", false, "Cancel quiet mode")

	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(whoamiCmd)
	Cmd.AddCommand(restartCmd)
	Cmd.AddCommand(quietCmd)
}
