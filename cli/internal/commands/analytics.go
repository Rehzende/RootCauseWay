package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Rehzende/RootCauseway/cli/internal/client"
	"github.com/spf13/cobra"
)

var analyticsCmd = &cobra.Command{
	Use:   "analytics",
	Short: "View analytics and metrics",
}

func init() {
	rootCmd.AddCommand(analyticsCmd)
	analyticsCmd.AddCommand(analyticsCostCmd)
	analyticsCmd.AddCommand(analyticsMTTRCmd)

	analyticsCostCmd.Flags().String("period", "7d", "Time period (e.g. 7d, 30d, 90d)")
	analyticsMTTRCmd.Flags().String("period", "30d", "Time period (e.g. 7d, 30d, 90d)")
}

var analyticsCostCmd = &cobra.Command{
	Use:   "cost",
	Short: "Show cost analytics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		period, _ := cmd.Flags().GetString("period")
		resp, err := c.Get("/api/v1/analytics/cost?period=" + period)
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

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}

		fmt.Printf("Cost Analytics (period: %s)\n\n", period)

		// Print top-level summary fields
		for _, key := range []string{"total_cost", "average_daily_cost", "projected_monthly"} {
			if v, ok := data[key]; ok {
				printField(key, v)
			}
		}

		// Print breakdown if available
		if breakdown, ok := data["breakdown"]; ok {
			fmt.Println("\nBreakdown:")
			if items, ok := breakdown.([]interface{}); ok {
				t := newTable(os.Stdout, []string{"ITEM", "COST"})
				for _, item := range items {
					if m, ok := item.(map[string]interface{}); ok {
						t.append([]string{
							fmt.Sprintf("%v", m["name"]),
							fmt.Sprintf("%v", m["cost"]),
						})
					}
				}
				t.render()
			} else {
				formatted, _ := json.MarshalIndent(breakdown, "", "  ")
				fmt.Println(string(formatted))
			}
		}

		return nil
	},
}

var analyticsMTTRCmd = &cobra.Command{
	Use:   "mttr",
	Short: "Show Mean Time To Resolution metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := client.NewFromConfig()
		if err != nil {
			return err
		}

		period, _ := cmd.Flags().GetString("period")
		resp, err := c.Get("/api/v1/analytics/mttr?period=" + period)
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

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}

		fmt.Printf("MTTR Analytics (period: %s)\n\n", period)

		for _, key := range []string{"average_mttr", "median_mttr", "p95_mttr", "total_incidents", "resolved_incidents"} {
			if v, ok := data[key]; ok {
				printField(key, v)
			}
		}

		// Print by severity if available
		if bySev, ok := data["by_severity"]; ok {
			fmt.Println("\nBy Severity:")
			formatted, _ := json.MarshalIndent(bySev, "", "  ")
			fmt.Println(string(formatted))
		}

		return nil
	},
}
