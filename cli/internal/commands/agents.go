package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "Manage AI agents",
}

func init() {
	rootCmd.AddCommand(agentsCmd)
	agentsCmd.AddCommand(agentsListCmd)
	agentsCmd.AddCommand(agentsGetCmd)
}

var agentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Get("/api/v1/agents")
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
				for _, key := range []string{"agents", "data", "items"} {
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

		for _, a := range items {
			t.append([]string{
				fmt.Sprintf("%v", a["id"]),
				fmt.Sprintf("%v", a["name"]),
				fmt.Sprintf("%v", a["type"]),
				fmt.Sprintf("%v", a["status"]),
			})
		}
		t.render()
		return nil
	},
}

var agentsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Get("/api/v1/agents/" + args[0])
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

		var a map[string]interface{}
		if err := json.Unmarshal(body, &a); err != nil {
			return err
		}

		printField("ID", a["id"])
		printField("Name", a["name"])
		printField("Type", a["type"])
		printField("Status", a["status"])
		printField("Description", a["description"])
		printField("Model", a["model"])
		printField("Created", a["created_at"])
		return nil
	},
}
