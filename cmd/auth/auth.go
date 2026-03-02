package auth

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
	Long:  `Log in with OAuth, set a bearer token, or view authentication status.`,
}

func init() {
	Cmd.AddCommand(loginCmd)
	Cmd.AddCommand(tokenCmd)
	Cmd.AddCommand(statusCmd)
	Cmd.AddCommand(logoutCmd)
	Cmd.AddCommand(refreshCmd)
}
