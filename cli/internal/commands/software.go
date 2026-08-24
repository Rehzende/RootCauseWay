package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

var softwareCmd = &cobra.Command{
	Use:     "software",
	Aliases: []string{"sw"},
	Short:   "Manage software catalog",
}

func init() {
	rootCmd.AddCommand(softwareCmd)
	softwareCmd.AddCommand(softwareListCmd)
	softwareCmd.AddCommand(softwareGetCmd)
}

var softwareListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered software",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Get("/api/v1/software")
		if err != nil {
			return err
		}
		if err := client.CheckResponse(resp); err != nil {
			return err
		}
		body, err := client.ReadBody(resp)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(body))
			return nil
		}

		var items []map[string]interface{}
		if err := json.Unmarshal(body, &items); err != nil {
			var wrapper map[string]json.RawMessage
			if err2 := json.Unmarshal(body, &wrapper); err2 == nil {
				for _, key := range []string{"software", "data", "items"} {
					if raw, ok := wrapper[key]; ok {
						_ = json.Unmarshal(raw, &items)
						break
					}
				}
			}
			if items == nil {
				return fmt.Errorf("failed to parse response: %w", err)
			}
		}

		t := newTable(os.Stdout, []string{"ID", "NAME", "TYPE", "STATUS"})

		for _, sw := range items {
			t.append([]string{
				fmt.Sprintf("%v", sw["id"]),
				fmt.Sprintf("%v", sw["name"]),
				fmt.Sprintf("%v", sw["type"]),
				fmt.Sprintf("%v", sw["status"]),
			})
		}
		t.render()
		return nil
	},
}

var softwareGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get software details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Get("/api/v1/software/" + args[0])
		if err != nil {
			return err
		}
		if err := client.CheckResponse(resp); err != nil {
			return err
		}
		body, err := client.ReadBody(resp)
		if err != nil {
			return err
		}

		if jsonOutput {
			fmt.Println(string(body))
			return nil
		}

		var sw map[string]interface{}
		if err := json.Unmarshal(body, &sw); err != nil {
			return err
		}

		printField("ID", sw["id"])
		printField("Name", sw["name"])
		printField("Type", sw["type"])
		printField("Status", sw["status"])
		printField("Description", sw["description"])
		printField("Repository", sw["repository_url"])
		printField("Created", sw["created_at"])
		return nil
	},
}
