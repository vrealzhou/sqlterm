package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

// ExampleUsageTracking demonstrates how to use the new usage tracking system
func ExampleUsageTracking(manager *Manager) {
	if manager.GetUsageStore() == nil {
		fmt.Println("Usage store not initialized. Initialize vector store first.")
		return
	}

	usageStore := manager.GetUsageStore()

	// Get today's usage
	fmt.Println("=== Today's Usage ===")
	todayUsage, err := usageStore.GetTodayUsage()
	if err != nil {
		fmt.Printf("Error getting today's usage: %v\n", err)
		return
	}

	if len(todayUsage) == 0 {
		fmt.Println("No usage recorded today.")
	} else {
		for i, usage := range todayUsage {
			if i >= 5 { // Show only first 5 entries
				fmt.Printf("... and %d more entries\n", len(todayUsage)-5)
				break
			}
			fmt.Printf("- %s: %s/%s - Input: %d, Output: %d, Cost: $%.6f\n",
				usage.RequestTime.Format("15:04:05"),
				usage.Provider, usage.Model,
				usage.InputTokens, usage.OutputTokens, usage.Cost)
		}
	}

	// Get usage summary
	fmt.Println("\n=== Usage Summary ===")
	summary, err := usageStore.GetUsageSummary()
	if err != nil {
		fmt.Printf("Error getting usage summary: %v\n", err)
		return
	}

	// Pretty print the summary
	summaryJSON, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println(string(summaryJSON))

	// Get provider/model breakdown for last 7 days
	fmt.Println("\n=== Provider/Model Breakdown (Last 7 Days) ===")
	breakdown, err := usageStore.GetProviderModelStats(7)
	if err != nil {
		fmt.Printf("Error getting provider breakdown: %v\n", err)
		return
	}

	for provider, models := range breakdown {
		fmt.Printf("\n%s:\n", provider)
		for model, stats := range models {
			if statsMap, ok := stats.(map[string]interface{}); ok {
				fmt.Printf("  %s: %d requests, $%.6f cost\n",
					model, int(statsMap["requests"].(int)), statsMap["cost"].(float64))
			}
		}
	}

	// Export usage data example
	fmt.Println("\n=== Export Example ===")
	startDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	endDate := time.Now().Format("2006-01-02")
	
	csvData, err := usageStore.ExportUsageData("csv", startDate, endDate)
	if err != nil {
		fmt.Printf("Error exporting CSV: %v\n", err)
	} else {
		fmt.Printf("CSV Export (%d bytes):\n%s\n", len(csvData), string(csvData))
	}
}

// ShowUsageStatistics is a convenience function to display current usage stats
func ShowUsageStatistics(manager *Manager) error {
	usageStore := manager.GetUsageStore()
	if usageStore == nil {
		return fmt.Errorf("usage tracking not available - no database connection")
	}

	summary, err := usageStore.GetUsageSummary()
	if err != nil {
		return fmt.Errorf("failed to get usage summary: %w", err)
	}

	fmt.Println("📊 Usage Statistics:")
	fmt.Printf("Session ID: %s\n", manager.GetSessionID())
	
	if todayStats, ok := summary["today"]; ok {
		if today, ok := todayStats.(map[string]interface{}); ok {
			fmt.Printf("Today: %d requests, %d input tokens, %d output tokens, $%.6f\n",
				int(today["requests"].(int)),
				int(today["input_tokens"].(int)),
				int(today["output_tokens"].(int)),
				today["cost"].(float64))
		}
	}

	if weekStats, ok := summary["last_7_days"]; ok {
		if week, ok := weekStats.(map[string]interface{}); ok {
			fmt.Printf("Last 7 days: %d requests, %d input tokens, %d output tokens, $%.6f\n",
				int(week["requests"].(int)),
				int(week["input_tokens"].(int)),
				int(week["output_tokens"].(int)),
				week["cost"].(float64))
		}
	}

	return nil
}