package api

import (
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/PhilipKram/jenkins-cli/internal/clientutil"
	"github.com/PhilipKram/jenkins-cli/internal/jenkins"
	"github.com/spf13/cobra"
)

func newClient(cmd *cobra.Command) (*jenkins.Client, error) {
	timeout, _ := cmd.Root().Flags().GetDuration("timeout")
	retries, _ := cmd.Root().Flags().GetInt("retries")
	return clientutil.NewClient(timeout, retries)
}

// Cmd is the top-level `api` command for raw API access.
var Cmd = &cobra.Command{
	Use:   "api <path>",
	Short: "Make an authenticated API request to Jenkins",
	Long: `Make an authenticated API request to any Jenkins endpoint.

The path is relative to the Jenkins base URL. CSRF crumb is automatically
included for POST/PUT/DELETE requests.`,
	Example: `  jenkins-cli api /api/json
  jenkins-cli api /job/my-job/api/json
  jenkins-cli api -X POST /job/my-job/build
  jenkins-cli api -X POST /createItem?name=new-job -d @config.xml
  jenkins-cli api -X POST /job/my-job/buildWithParameters -f BRANCH=main -f ENV=prod`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		method, _ := cmd.Flags().GetString("method")
		data, _ := cmd.Flags().GetString("data")
		fields, _ := cmd.Flags().GetStringArray("field")

		client, err := newClient(cmd)
		if err != nil {
			return err
		}

		var body io.Reader
		if len(fields) > 0 {
			values := url.Values{}
			for _, f := range fields {
				parts := strings.SplitN(f, "=", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid field format %q, expected key=value", f)
				}
				values.Add(parts[0], parts[1])
			}
			body = strings.NewReader(values.Encode())
		} else if data != "" {
			if strings.HasPrefix(data, "@") {
				f, err := os.Open(data[1:])
				if err != nil {
					return fmt.Errorf("opening file: %w", err)
				}
				defer f.Close()
				body = f
			} else {
				body = strings.NewReader(data)
			}
		}

		ctx := cmd.Context()
		resp, err := client.RawRequest(ctx, method, path, body)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			respBody, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		_, err = io.Copy(os.Stdout, resp.Body)
		return err
	},
}

func init() {
	Cmd.Flags().StringP("method", "X", "GET", "HTTP method")
	Cmd.Flags().StringP("data", "d", "", "Request body (use @filename to read from file)")
	Cmd.Flags().StringArrayP("field", "f", nil, "Form fields as key=value (repeatable)")
}
