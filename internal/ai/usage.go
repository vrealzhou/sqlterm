package ai

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sqlterm/internal/config"
	"time"
)

// UsageDetails represents a single LLM request with detailed information
type UsageDetails struct {
	ID           int             `json:"id"`
	SessionID    string          `json:"session_id"`
	Provider     config.Provider `json:"provider"`
	Model        string          `json:"model"`
	InputTokens  int             `json:"input_tokens"`
	OutputTokens int             `json:"output_tokens"`
	Cost         float64         `json:"cost"`
	RequestTime  time.Time       `json:"request_time"`
	UserMessage  string          `json:"user_message"`
	AIResponse   string          `json:"ai_response"`
}

// DailyUsageStats represents aggregated daily statistics per provider/model
type DailyUsageStats struct {
	ID           int             `json:"id"`
	Date         string          `json:"date"` // YYYY-MM-DD format
	Provider     config.Provider `json:"provider"`
	Model        string          `json:"model"`
	TotalRequests int            `json:"total_requests"`
	InputTokens  int             `json:"input_tokens"`
	OutputTokens int             `json:"output_tokens"`
	TotalCost    float64         `json:"total_cost"`
	CreatedAt    time.Time       `json:"created_at"`
}

// UsageStore manages usage tracking in the vector database
type UsageStore struct {
	db             *sql.DB
	lastProcessedDate string
}

// NewUsageStore creates a new usage store or gets existing one from vector store
func NewUsageStore(vectorStore *VectorStore) (*UsageStore, error) {
	store := &UsageStore{
		db: vectorStore.db,
	}

	if err := store.initializeUsageSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize usage schema: %w", err)
	}

	// Truncate current day details if it's a new day
	if err := store.handleDayChange(); err != nil {
		return nil, fmt.Errorf("failed to handle day change: %w", err)
	}

	return store, nil
}

// initializeUsageSchema creates the usage tracking tables
func (us *UsageStore) initializeUsageSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS usage_details (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			input_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			cost REAL NOT NULL,
			request_time DATETIME NOT NULL,
			user_message TEXT,
			ai_response TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS daily_usage_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT NOT NULL,
			provider TEXT NOT NULL,
			model TEXT NOT NULL,
			total_requests INTEGER NOT NULL,
			input_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			total_cost REAL NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(date, provider, model)
		)`,

		`CREATE INDEX IF NOT EXISTS idx_usage_details_date ON usage_details(date(request_time))`,
		`CREATE INDEX IF NOT EXISTS idx_usage_details_provider ON usage_details(provider, model)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_details_session ON usage_details(session_id)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_usage_stats(date DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_daily_stats_provider ON daily_usage_stats(provider, model)`,
	}

	for _, query := range queries {
		if _, err := us.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute usage schema query: %w", err)
		}
	}

	return nil
}

// handleDayChange processes statistics when date changes and truncates current day details
func (us *UsageStore) handleDayChange() error {
	currentDate := time.Now().Format("2006-01-02")
	
	// Check if we need to process the previous day's data
	var lastDate sql.NullString
	err := us.db.QueryRow(`SELECT MAX(date(request_time)) FROM usage_details`).Scan(&lastDate)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get last usage date: %w", err)
	}

	// If there's data from previous days that hasn't been aggregated, process it
	if lastDate.Valid && lastDate.String != currentDate {
		if err := us.aggregateDailyStats(lastDate.String); err != nil {
			return fmt.Errorf("failed to aggregate daily stats: %w", err)
		}
		
		// Clean up old usage details (keep only current day)
		if err := us.truncateOldDetails(currentDate); err != nil {
			return fmt.Errorf("failed to truncate old details: %w", err)
		}
	}

	us.lastProcessedDate = currentDate
	return nil
}

// RecordUsage records a new usage entry
func (us *UsageStore) RecordUsage(sessionID string, provider config.Provider, model string, 
	inputTokens, outputTokens int, cost float64, userMessage, aiResponse string) error {
	
	query := `INSERT INTO usage_details 
		(session_id, provider, model, input_tokens, output_tokens, cost, request_time, user_message, ai_response)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := us.db.Exec(query, sessionID, string(provider), model, inputTokens, outputTokens, 
		cost, time.Now(), userMessage, aiResponse)

	if err != nil {
		return fmt.Errorf("failed to record usage: %w", err)
	}

	// Check if date has changed and handle accordingly
	currentDate := time.Now().Format("2006-01-02")
	if us.lastProcessedDate != currentDate {
		if err := us.handleDayChange(); err != nil {
			// Log error but don't fail the recording
			fmt.Printf("Warning: failed to handle day change: %v\n", err)
		}
	}

	return nil
}

// aggregateDailyStats calculates and stores daily statistics for a specific date
func (us *UsageStore) aggregateDailyStats(date string) error {
	query := `INSERT OR REPLACE INTO daily_usage_stats 
		(date, provider, model, total_requests, input_tokens, output_tokens, total_cost)
		SELECT 
			date(request_time) as date,
			provider,
			model,
			COUNT(*) as total_requests,
			SUM(input_tokens) as input_tokens,
			SUM(output_tokens) as output_tokens,
			SUM(cost) as total_cost
		FROM usage_details 
		WHERE date(request_time) = ?
		GROUP BY date(request_time), provider, model`

	_, err := us.db.Exec(query, date)
	return err
}

// truncateOldDetails removes usage details from previous days, keeping only current day
func (us *UsageStore) truncateOldDetails(currentDate string) error {
	query := `DELETE FROM usage_details WHERE date(request_time) < ?`
	_, err := us.db.Exec(query, currentDate)
	return err
}

// GetTodayUsage returns today's usage details
func (us *UsageStore) GetTodayUsage() ([]UsageDetails, error) {
	currentDate := time.Now().Format("2006-01-02")
	
	query := `SELECT id, session_id, provider, model, input_tokens, output_tokens, 
		cost, request_time, user_message, ai_response
		FROM usage_details 
		WHERE date(request_time) = ?
		ORDER BY request_time DESC`

	rows, err := us.db.Query(query, currentDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's usage: %w", err)
	}
	defer rows.Close()

	var usageList []UsageDetails
	for rows.Next() {
		var usage UsageDetails
		var provider string
		err := rows.Scan(&usage.ID, &usage.SessionID, &provider, &usage.Model,
			&usage.InputTokens, &usage.OutputTokens, &usage.Cost, &usage.RequestTime,
			&usage.UserMessage, &usage.AIResponse)
		if err != nil {
			continue
		}
		usage.Provider = config.Provider(provider)
		usageList = append(usageList, usage)
	}

	return usageList, nil
}

// GetDailyStats returns daily aggregated statistics for a date range
func (us *UsageStore) GetDailyStats(startDate, endDate string) ([]DailyUsageStats, error) {
	query := `SELECT id, date, provider, model, total_requests, input_tokens, 
		output_tokens, total_cost, created_at
		FROM daily_usage_stats 
		WHERE date BETWEEN ? AND ?
		ORDER BY date DESC, provider, model`

	rows, err := us.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily stats: %w", err)
	}
	defer rows.Close()

	var statsList []DailyUsageStats
	for rows.Next() {
		var stats DailyUsageStats
		var provider string
		err := rows.Scan(&stats.ID, &stats.Date, &provider, &stats.Model,
			&stats.TotalRequests, &stats.InputTokens, &stats.OutputTokens,
			&stats.TotalCost, &stats.CreatedAt)
		if err != nil {
			continue
		}
		stats.Provider = config.Provider(provider)
		statsList = append(statsList, stats)
	}

	return statsList, nil
}

// GetUsageSummary returns a summary of usage for today and recent days
func (us *UsageStore) GetUsageSummary() (map[string]interface{}, error) {
	currentDate := time.Now().Format("2006-01-02")
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")

	// Get today's totals
	todayQuery := `SELECT 
		COUNT(*) as requests,
		COALESCE(SUM(input_tokens), 0) as input_tokens,
		COALESCE(SUM(output_tokens), 0) as output_tokens,
		COALESCE(SUM(cost), 0) as cost
		FROM usage_details 
		WHERE date(request_time) = ?`

	var todayStats struct {
		Requests     int     `json:"requests"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		Cost         float64 `json:"cost"`
	}

	err := us.db.QueryRow(todayQuery, currentDate).Scan(
		&todayStats.Requests, &todayStats.InputTokens, 
		&todayStats.OutputTokens, &todayStats.Cost)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get today's summary: %w", err)
	}

	// Get last 7 days from daily stats
	weekQuery := `SELECT 
		COALESCE(SUM(total_requests), 0) as requests,
		COALESCE(SUM(input_tokens), 0) as input_tokens,
		COALESCE(SUM(output_tokens), 0) as output_tokens,
		COALESCE(SUM(total_cost), 0) as cost
		FROM daily_usage_stats 
		WHERE date BETWEEN ? AND ?`

	var weekStats struct {
		Requests     int     `json:"requests"`
		InputTokens  int     `json:"input_tokens"`
		OutputTokens int     `json:"output_tokens"`
		Cost         float64 `json:"cost"`
	}

	err = us.db.QueryRow(weekQuery, weekAgo, currentDate).Scan(
		&weekStats.Requests, &weekStats.InputTokens, 
		&weekStats.OutputTokens, &weekStats.Cost)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get week summary: %w", err)
	}

	// Combine today's details with week totals (avoid double counting)
	summary := map[string]interface{}{
		"today": todayStats,
		"last_7_days": map[string]interface{}{
			"requests":     weekStats.Requests + todayStats.Requests,
			"input_tokens": weekStats.InputTokens + todayStats.InputTokens,
			"output_tokens": weekStats.OutputTokens + todayStats.OutputTokens,
			"cost":         weekStats.Cost + todayStats.Cost,
		},
	}

	return summary, nil
}

// ExportUsageData exports usage data in different formats
func (us *UsageStore) ExportUsageData(format string, startDate, endDate string) ([]byte, error) {
	// Get daily stats for the period
	stats, err := us.GetDailyStats(startDate, endDate)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return json.MarshalIndent(stats, "", "  ")
	case "csv":
		return us.exportCSV(stats)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// exportCSV converts usage stats to CSV format
func (us *UsageStore) exportCSV(stats []DailyUsageStats) ([]byte, error) {
	csv := "Date,Provider,Model,Total Requests,Input Tokens,Output Tokens,Total Cost\n"
	
	for _, stat := range stats {
		csv += fmt.Sprintf("%s,%s,%s,%d,%d,%d,%.6f\n",
			stat.Date, stat.Provider, stat.Model, stat.TotalRequests,
			stat.InputTokens, stat.OutputTokens, stat.TotalCost)
	}
	
	return []byte(csv), nil
}

// GetProviderModelStats returns usage breakdown by provider and model
func (us *UsageStore) GetProviderModelStats(days int) (map[string]map[string]interface{}, error) {
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	currentDate := time.Now().Format("2006-01-02")

	query := `SELECT provider, model,
		COALESCE(SUM(total_requests), 0) as requests,
		COALESCE(SUM(input_tokens), 0) as input_tokens,
		COALESCE(SUM(output_tokens), 0) as output_tokens,
		COALESCE(SUM(total_cost), 0) as cost
		FROM daily_usage_stats 
		WHERE date BETWEEN ? AND ?
		GROUP BY provider, model
		ORDER BY cost DESC`

	rows, err := us.db.Query(query, startDate, currentDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider/model stats: %w", err)
	}
	defer rows.Close()

	result := make(map[string]map[string]interface{})
	
	for rows.Next() {
		var provider, model string
		var requests, inputTokens, outputTokens int
		var cost float64

		err := rows.Scan(&provider, &model, &requests, &inputTokens, &outputTokens, &cost)
		if err != nil {
			continue
		}

		if result[provider] == nil {
			result[provider] = make(map[string]interface{})
		}

		result[provider][model] = map[string]interface{}{
			"requests":      requests,
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"cost":          cost,
		}
	}

	// Add today's data from usage_details
	todayQuery := `SELECT provider, model,
		COUNT(*) as requests,
		COALESCE(SUM(input_tokens), 0) as input_tokens,
		COALESCE(SUM(output_tokens), 0) as output_tokens,
		COALESCE(SUM(cost), 0) as cost
		FROM usage_details 
		WHERE date(request_time) = ?
		GROUP BY provider, model`

	todayRows, err := us.db.Query(todayQuery, currentDate)
	if err == nil {
		defer todayRows.Close()
		
		for todayRows.Next() {
			var provider, model string
			var requests, inputTokens, outputTokens int
			var cost float64

			err := todayRows.Scan(&provider, &model, &requests, &inputTokens, &outputTokens, &cost)
			if err != nil {
				continue
			}

			if result[provider] == nil {
				result[provider] = make(map[string]interface{})
			}

			// Add today's data to existing stats
			if existing, exists := result[provider][model]; exists {
				if existingMap, ok := existing.(map[string]interface{}); ok {
					existingMap["requests"] = existingMap["requests"].(int) + requests
					existingMap["input_tokens"] = existingMap["input_tokens"].(int) + inputTokens
					existingMap["output_tokens"] = existingMap["output_tokens"].(int) + outputTokens
					existingMap["cost"] = existingMap["cost"].(float64) + cost
				}
			} else {
				result[provider][model] = map[string]interface{}{
					"requests":      requests,
					"input_tokens":  inputTokens,
					"output_tokens": outputTokens,
					"cost":          cost,
				}
			}
		}
	}

	return result, nil
}