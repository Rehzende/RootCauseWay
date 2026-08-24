package commands

import (
	"encoding/json"
	"fmt"

	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(loginCmd)
	rootCmd.AddCommand(logoutCmd)
	rootCmd.AddCommand(whoamiCmd)

	loginCmd.Flags().String("email", "", "Email address")
	loginCmd.Flags().String("password", "", "Password")
	_ = loginCmd.MarkFlagRequired("email")
	_ = loginCmd.MarkFlagRequired("password")
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with the RootCauseway API",
	RunE: func(cmd *cobra.Command, args []string) error {
		email, _ := cmd.Flags().GetString("email")
		password, _ := cmd.Flags().GetString("password")

		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Post("/api/v1/auth/login", map[string]string{
			"email":    email,
			"password": password,
		})
		if err != nil {
			return fmt.Errorf("login request failed: %w", err)
		}
		if err := client.CheckResponse(resp); err != nil {
			return err
		}

		body, err := client.ReadBody(resp)
		if err != nil {
			return err
		}

		var result struct {
			Token string `json:"token"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return fmt.Errorf("failed to parse login response: %w", err)
		}

		cfg, _ := client.LoadConfig()
		cfg.Token = result.Token
		if err := client.SaveConfig(cfg); err != nil {
			return fmt.Errorf("failed to save token: %w", err)
		}

		fmt.Println("Login successful. Token saved.")
		return nil
	},
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Clear saved authentication token",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := client.LoadConfig()
		cfg.Token = ""
		if err := client.SaveConfig(cfg); err != nil {
			return err
		}
		fmt.Println("Logged out. Token cleared.")
		return nil
	},
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Show the currently authenticated user",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}
		if c.Token == "" {
			return fmt.Errorf("not logged in — run 'rootcauseway login' first")
		}

		resp, err := c.Get("/api/v1/auth/me")
		if err != nil {
			return fmt.Errorf("request failed: %w", err)
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

		var user struct {
			ID    interface{} `json:"id"`
			Email string      `json:"email"`
			Name  string      `json:"name"`
			Role  string      `json:"role"`
		}
		if err := json.Unmarshal(body, &user); err != nil {
			return err
		}

		fmt.Printf("ID:    %v\n", user.ID)
		fmt.Printf("Email: %s\n", user.Email)
		fmt.Printf("Name:  %s\n", user.Name)
		fmt.Printf("Role:  %s\n", user.Role)
		return nil
	},
}
