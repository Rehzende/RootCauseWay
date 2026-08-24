package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage CLI configuration",
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configShowCmd)
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value (api-url, token)",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := client.LoadConfig()
		if err != nil {
			return err
		}

		key, value := args[0], args[1]
		switch key {
		case "api-url":
			cfg.APIURL = value
		case "token":
			cfg.Token = value
		default:
			return fmt.Errorf("unknown config key %q (valid: api-url, token)", key)
		}

		if err := client.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Set %s = %s\n", key, value)
		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := client.LoadConfig()
		if err != nil {
			return err
		}

		if jsonOutput {
			data, _ := json.MarshalIndent(cfg, "", "  ")
			fmt.Println(string(data))
			return nil
		}

		fmt.Printf("API URL: %s\n", cfg.APIURL)
		token := cfg.Token
		if len(token) > 20 {
			token = token[:10] + "..." + token[len(token)-10:]
		}
		if token == "" {
			token = "(not set)"
		}
		fmt.Printf("Token:   %s\n", token)
		return nil
	},
}
