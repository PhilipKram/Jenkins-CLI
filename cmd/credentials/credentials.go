package credentials

import (
	"bufio"
	"fmt"
	"os"
	"strings"

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
	Use:   "credentials",
	Short: "Manage Jenkins credentials",
}

var listCmd = &cobra.Command{
	Use:   "list [domain]",
	Short: "List all credentials",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		domain := ""
		if len(args) > 0 {
			domain = args[0]
		}

		credentials, err := client.ListCredentials(ctx, domain)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, credentials)
		}

		headers := []string{"ID", "TYPE", "DISPLAY NAME", "DESCRIPTION"}
		rows := make([][]string, len(credentials))
		for i, cred := range credentials {
			rows[i] = []string{cred.ID, cred.Type, cred.DisplayName, cred.Description}
		}
		output.PrintTable(os.Stdout, headers, rows)
		return nil
	},
}

var infoCmd = &cobra.Command{
	Use:   "info <credential-id>",
	Short: "Show detailed credential information",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		domain, _ := cmd.Flags().GetString("domain")
		cred, err := client.GetCredential(ctx, args[0], domain)
		if err != nil {
			return err
		}

		jsonOut, _ := cmd.Flags().GetBool("json")
		if jsonOut {
			return output.PrintJSON(os.Stdout, cred)
		}

		fmt.Printf("ID:           %s\n", cred.ID)
		fmt.Printf("Type:         %s\n", cred.Type)
		fmt.Printf("Display Name: %s\n", cred.DisplayName)
		if cred.Description != "" {
			fmt.Printf("Description:  %s\n", cred.Description)
		}
		fmt.Printf("Scope:        %s\n", cred.Scope)
		fmt.Printf("Domain:       %s\n", cred.Domain)
		if cred.Fingerprint != "" {
			fmt.Printf("Fingerprint:  %s\n", cred.Fingerprint)
		}
		return nil
	},
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new credential",
	Long:  `Create a new credential with interactive prompts for different credential types.`,
	RunE:  runCreate,
}

func runCreate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	client, err := newClient(cmd)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(os.Stdin)

	// Get credential type
	credType, _ := cmd.Flags().GetString("type")
	if credType == "" {
		fmt.Println("Select credential type:")
		fmt.Println("  1) username-password")
		fmt.Println("  2) secret-text")
		fmt.Println("  3) ssh-key")
		fmt.Println("  4) certificate")
		credType = prompt(reader, "Type [1-4]", "1")
		switch credType {
		case "1":
			credType = "username-password"
		case "2":
			credType = "secret-text"
		case "3":
			credType = "ssh-key"
		case "4":
			credType = "certificate"
		default:
			return fmt.Errorf("invalid credential type selection")
		}
	}

	// Common fields
	id := prompt(reader, "Credential ID", "")
	if id == "" {
		return fmt.Errorf("credential ID is required")
	}

	description := prompt(reader, "Description", "")

	domain, _ := cmd.Flags().GetString("domain")
	scope := prompt(reader, "Scope (GLOBAL/SYSTEM)", "GLOBAL")
	scope = strings.ToUpper(scope)

	// Build payload based on type
	payload := jenkins.CredentialPayload{
		ID:          id,
		Scope:       scope,
		Description: description,
	}

	switch credType {
	case "username-password":
		payload.Username = prompt(reader, "Username", "")
		if payload.Username == "" {
			return fmt.Errorf("username is required")
		}
		payload.Password = prompt(reader, "Password", "")
		if payload.Password == "" {
			return fmt.Errorf("password is required")
		}

	case "secret-text":
		payload.Secret = prompt(reader, "Secret", "")
		if payload.Secret == "" {
			return fmt.Errorf("secret is required")
		}

	case "ssh-key":
		payload.Username = prompt(reader, "Username", "")
		if payload.Username == "" {
			return fmt.Errorf("username is required")
		}
		fmt.Println("Enter private key (end with a line containing only '---END---'):")
		var keyLines []string
		for {
			line, _ := reader.ReadString('\n')
			line = strings.TrimRight(line, "\r\n")
			if line == "---END---" {
				break
			}
			keyLines = append(keyLines, line)
		}
		payload.PrivateKey = strings.Join(keyLines, "\n")
		if payload.PrivateKey == "" {
			return fmt.Errorf("private key is required")
		}
		payload.Passphrase = prompt(reader, "Passphrase (optional)", "")

	case "certificate":
		return fmt.Errorf("certificate credential type is not yet fully supported")

	default:
		return fmt.Errorf("unsupported credential type: %s", credType)
	}

	// Create the credential
	if err := client.CreateCredential(ctx, credType, domain, payload); err != nil {
		return err
	}

	fmt.Printf("Credential '%s' created successfully\n", id)
	return nil
}

var deleteCmd = &cobra.Command{
	Use:   "delete <credential-id>",
	Short: "Delete a credential",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		credID := args[0]
		domain, _ := cmd.Flags().GetString("domain")

		// Confirmation prompt
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("Are you sure you want to delete credential %q? (yes/no): ", credID)
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(strings.ToLower(confirmation))

		if confirmation != "yes" && confirmation != "y" {
			fmt.Println("Delete cancelled")
			return nil
		}

		if err := client.DeleteCredential(ctx, credID, domain); err != nil {
			return err
		}
		fmt.Printf("Credential %q deleted\n", credID)
		return nil
	},
}

func prompt(reader *bufio.Reader, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
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

func init() {
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().Bool("json", false, "Output in JSON format")
	infoCmd.Flags().StringP("domain", "d", "", "Credential domain (default: global)")

	createCmd.Flags().StringP("type", "t", "", "Credential type: username-password, secret-text, ssh-key, certificate")
	createCmd.Flags().StringP("domain", "d", "", "Credential domain (default: global)")

	deleteCmd.Flags().StringP("domain", "d", "", "Credential domain (default: global)")

	Cmd.AddCommand(listCmd)
	Cmd.AddCommand(infoCmd)
	Cmd.AddCommand(createCmd)
	Cmd.AddCommand(deleteCmd)
}
