package profile

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/PhilipKram/jenkins-cli/internal/config"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "profile",
	Short: "Manage configuration profiles for multiple Jenkins servers",
	Long: `Manage named configuration profiles, each pointing to a different Jenkins server
with its own credentials.

Create a profile:
  jenkins-cli configure --profile images --url https://jenkins.example.com/images

Switch profiles:
  jenkins-cli profile use images

Use a profile for a single command:
  jenkins-cli --profile images jobs list`,
}

func init() {
	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(useCmd)
	Cmd.AddCommand(deleteCmd)
	Cmd.AddCommand(showCmd)
}

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List all configured profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, err := config.LoadMulti()
		if err != nil {
			return err
		}

		names := config.ProfileNames(mc)
		if len(names) == 0 {
			fmt.Println("No profiles configured. Run 'jenkins-cli configure' to create one.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PROFILE\tURL\tAUTH\tACTIVE")
		for _, name := range names {
			p := mc.Profiles[name]
			active := ""
			if name == mc.CurrentProfile {
				active = "*"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", name, p.URL, p.EffectiveAuthType(), active)
		}
		return w.Flush()
	},
}

var useCmd = &cobra.Command{
	Use:   "use <profile-name>",
	Short: "Set the active profile",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		mc, err := config.LoadMulti()
		if err != nil {
			return err
		}

		if _, ok := mc.Profiles[name]; !ok {
			return fmt.Errorf("profile %q not found. Run 'jenkins-cli profile list' to see available profiles", name)
		}

		mc.CurrentProfile = name
		if err := config.SaveMulti(mc); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Switched to profile %q (%s)\n", name, mc.Profiles[name].URL)
		return nil
	},
}

var deleteCmd = &cobra.Command{
	Use:     "delete <profile-name>",
	Aliases: []string{"rm", "remove"},
	Short:   "Delete a profile",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		mc, err := config.LoadMulti()
		if err != nil {
			return err
		}

		if _, ok := mc.Profiles[name]; !ok {
			return fmt.Errorf("profile %q not found", name)
		}

		if len(mc.Profiles) == 1 {
			return fmt.Errorf("cannot delete the only profile. Configure another profile first")
		}

		delete(mc.Profiles, name)

		// If we deleted the current profile, switch to another one
		if mc.CurrentProfile == name {
			for n := range mc.Profiles {
				mc.CurrentProfile = n
				break
			}
			fmt.Printf("Active profile switched to %q\n", mc.CurrentProfile)
		}

		if err := config.SaveMulti(mc); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("Profile %q deleted\n", name)
		return nil
	},
}

var showCmd = &cobra.Command{
	Use:   "show [profile-name]",
	Short: "Show details of a profile (defaults to active profile)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mc, err := config.LoadMulti()
		if err != nil {
			return err
		}

		var name string
		if len(args) > 0 {
			name = args[0]
		} else {
			// Use standard profile resolution: --profile flag > JENKINS_PROFILE env > current_profile
			resolved, err := config.CurrentProfileName()
			if err != nil {
				return err
			}
			name = resolved
		}

		p, ok := mc.Profiles[name]
		if !ok {
			return fmt.Errorf("profile %q not found", name)
		}

		active := ""
		if name == mc.CurrentProfile {
			active = " (active)"
		}
		fmt.Printf("Profile: %s%s\n", name, active)
		fmt.Printf("URL:     %s\n", p.URL)
		fmt.Printf("Auth:    %s\n", p.EffectiveAuthType())
		if p.User != "" {
			fmt.Printf("User:    %s\n", p.User)
		}
		if p.Insecure {
			fmt.Printf("TLS:     insecure (verification disabled)\n")
		}
		return nil
	},
}
