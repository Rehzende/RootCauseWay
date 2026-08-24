package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

var incidentsCmd = &cobra.Command{
	Use:     "incidents",
	Aliases: []string{"inc"},
	Short:   "Manage incidents",
}

func init() {
	rootCmd.AddCommand(incidentsCmd)
	incidentsCmd.AddCommand(incidentsListCmd)
	incidentsCmd.AddCommand(incidentsGetCmd)
	incidentsCmd.AddCommand(incidentsSendAlertCmd)

	incidentsListCmd.Flags().String("status", "", "Filter by status (open, investigating, resolved)")
	incidentsListCmd.Flags().String("severity", "", "Filter by severity (critical, high, medium, low)")
	incidentsListCmd.Flags().Int("limit", 20, "Max number of results")
	incidentsListCmd.Flags().Bool("wide", false, "Show additional columns")

	incidentsSendAlertCmd.Flags().String("webhook-token", "", "Webhook token for alert ingestion")
	incidentsSendAlertCmd.Flags().String("source", "", "Alert source (prometheus, datadog, grafana, custom)")
	incidentsSendAlertCmd.Flags().String("file", "", "JSON file containing the alert payload")
	incidentsSendAlertCmd.Flags().String("title", "", "Alert title (inline mode)")
	incidentsSendAlertCmd.Flags().String("severity", "", "Alert severity (inline mode)")
	incidentsSendAlertCmd.Flags().String("service", "", "Affected service (inline mode)")
	incidentsSendAlertCmd.Flags().String("description", "", "Alert description (inline mode)")
	_ = incidentsSendAlertCmd.MarkFlagRequired("webhook-token")
	_ = incidentsSendAlertCmd.MarkFlagRequired("source")
}

var incidentsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List incidents",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		var params []string
		if v, _ := cmd.Flags().GetString("status"); v != "" {
			params = append(params, "status="+v)
		}
		if v, _ := cmd.Flags().GetString("severity"); v != "" {
			params = append(params, "severity="+v)
		}
		if v, _ := cmd.Flags().GetInt("limit"); v > 0 {
			params = append(params, fmt.Sprintf("limit=%d", v))
		}

		path := "/api/v1/incidents"
		if len(params) > 0 {
			path += "?" + strings.Join(params, "&")
		}

		resp, err := c.Get(path)
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

		var incidents []map[string]interface{}
		if err := json.Unmarshal(body, &incidents); err != nil {
			// Try unwrapping from a wrapper object
			var wrapper map[string]json.RawMessage
			if err2 := json.Unmarshal(body, &wrapper); err2 == nil {
				for _, key := range []string{"incidents", "data", "items"} {
					if raw, ok := wrapper[key]; ok {
						_ = json.Unmarshal(raw, &incidents)
						break
					}
				}
			}
			if incidents == nil {
				return fmt.Errorf("failed to parse incidents: %w", err)
			}
		}

		wide, _ := cmd.Flags().GetBool("wide")
		headers := []string{"ID", "TITLE", "SEVERITY", "STATUS"}
		if wide {
			headers = append(headers, "SERVICE", "CREATED")
		}
		t := newTable(os.Stdout, headers)

		for _, inc := range incidents {
			id := fmt.Sprintf("%v", inc["id"])
			title := truncate(fmt.Sprintf("%v", inc["title"]), 50)
			sev := fmt.Sprintf("%v", inc["severity"])
			status := fmt.Sprintf("%v", inc["status"])

			row := []string{id, title, colorSeverity(sev), colorStatus(status)}
			if wide {
				service := fmt.Sprintf("%v", inc["service"])
				created := fmt.Sprintf("%v", inc["created_at"])
				row = append(row, service, created)
			}
			t.append(row)
		}
		t.render()
		return nil
	},
}

var incidentsGetCmd = &cobra.Command{
	Use:   "get <id>",
	Short: "Get incident details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		resp, err := c.Get("/api/v1/incidents/" + args[0])
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

		var inc map[string]interface{}
		if err := json.Unmarshal(body, &inc); err != nil {
			return err
		}

		printField("ID", inc["id"])
		printField("Title", inc["title"])
		printField("Severity", colorSeverity(fmt.Sprintf("%v", inc["severity"])))
		printField("Status", colorStatus(fmt.Sprintf("%v", inc["status"])))
		printField("Service", inc["service"])
		printField("Description", inc["description"])
		printField("Created", inc["created_at"])
		printField("Updated", inc["updated_at"])

		if rca, ok := inc["root_cause_analysis"]; ok && rca != nil {
			fmt.Println("\n--- Root Cause Analysis ---")
			rcaJSON, _ := json.MarshalIndent(rca, "", "  ")
			fmt.Println(string(rcaJSON))
		}

		return nil
	},
}

var incidentsSendAlertCmd = &cobra.Command{
	Use:   "send-alert",
	Short: "Send an alert to create or update an incident",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		webhookToken, _ := cmd.Flags().GetString("webhook-token")
		source, _ := cmd.Flags().GetString("source")
		file, _ := cmd.Flags().GetString("file")

		var payload interface{}

		if file != "" {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("failed to read alert file: %w", err)
			}
			if err := json.Unmarshal(data, &payload); err != nil {
				return fmt.Errorf("invalid JSON in alert file: %w", err)
			}
		} else {
			title, _ := cmd.Flags().GetString("title")
			severity, _ := cmd.Flags().GetString("severity")
			service, _ := cmd.Flags().GetString("service")
			description, _ := cmd.Flags().GetString("description")

			if title == "" {
				return fmt.Errorf("either --file or --title is required")
			}

			payload = map[string]string{
				"title":       title,
				"severity":    severity,
				"service":     service,
				"description": description,
			}
		}

		path := fmt.Sprintf("/api/v1/webhooks/%s/alerts?source=%s", webhookToken, source)
		resp, err := c.Post(path, payload)
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

		fmt.Println("Alert sent successfully.")
		var result map[string]interface{}
		if json.Unmarshal(body, &result) == nil {
			if id, ok := result["incident_id"]; ok {
				fmt.Printf("Incident ID: %v\n", id)
			}
		}
		return nil
	},
}

func colorSeverity(s string) string {
	switch strings.ToLower(s) {
	case "critical":
		return color.RedString(s)
	case "high":
		return color.HiRedString(s)
	case "medium":
		return color.YellowString(s)
	case "low":
		return color.GreenString(s)
	default:
		return s
	}
}

func colorStatus(s string) string {
	switch strings.ToLower(s) {
	case "open":
		return color.RedString(s)
	case "investigating":
		return color.YellowString(s)
	case "resolved":
		return color.GreenString(s)
	default:
		return s
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func printField(label string, value interface{}) {
	if value == nil {
		return
	}
	fmt.Printf("%-14s %v\n", label+":", value)
}
