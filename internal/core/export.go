package core

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (result *QueryResult) ToMarkdown(limit int) string {
	if len(result.Rows) == 0 {
		return "No results returned.\n"
	}

	var sb strings.Builder

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	for i, col := range result.Columns {
		widths[i] = len(col)
	}

	// Consider only the first 'limit' rows for width calculation
	rowsToProcess := result.Rows
	if limit > 0 && len(result.Rows) > limit {
		rowsToProcess = result.Rows[:limit]
	}

	for _, row := range rowsToProcess {
		for i, val := range row {
			if i < len(widths) && len(val.String()) > widths[i] {
				widths[i] = len(val.String())
			}
		}
	}

	// Write header
	sb.WriteString("| ")
	for i, col := range result.Columns {
		sb.WriteString(fmt.Sprintf("%-*s", widths[i], col))
		if i < len(result.Columns)-1 {
			sb.WriteString(" | ")
		}
	}
	sb.WriteString(" |\n")

	// Write separator
	sb.WriteString("|")
	for i := range result.Columns {
		sb.WriteString(strings.Repeat("-", widths[i]+2))
		if i < len(result.Columns)-1 {
			sb.WriteString("|")
		}
	}
	sb.WriteString("|\n")

	// Write rows (limited)
	for _, row := range rowsToProcess {
		sb.WriteString("| ")
		for i, val := range row {
			if i < len(widths) {
				sb.WriteString(fmt.Sprintf("%-*s", widths[i], val.String()))
			}
			if i < len(result.Columns)-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString(" |\n")
	}

	// Add truncation note if limited
	if limit > 0 && len(result.Rows) > limit {
		sb.WriteString(fmt.Sprintf("\n*Note: Showing top %d of %d rows. Use CSV export for complete results.*\n", limit, len(result.Rows)))
	}

	return sb.String()
}

func (result *QueryResult) ToCSV() (string, error) {
	var sb strings.Builder
	writer := csv.NewWriter(&sb)

	// Write headers
	if err := writer.Write(result.Columns); err != nil {
		return "", fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write rows
	for _, row := range result.Rows {
		record := make([]string, len(row))
		for i, val := range row {
			record[i] = val.String()
		}
		if err := writer.Write(record); err != nil {
			return "", fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("CSV writer error: %w", err)
	}

	return sb.String(), nil
}

func SaveQueryResultAsMarkdown(result *QueryResult, query string, connection string, resultWriter io.Writer) error {
	// Create markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("**Query:**\n```sql\n%s\n```\n\n", query))
	content.WriteString(fmt.Sprintf("**Results:** %d rows\n\n", len(result.Rows)))

	// Add the markdown table (limited to 20 rows)
	content.WriteString(result.ToMarkdown(20))
	content.WriteString("\n\n")

	// Write to file
	if _, err := resultWriter.Write([]byte(content.String())); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}

	return nil
}

func SaveQueryResultAsCSV(result *QueryResult, filePath string) error {
	csvContent, err := result.ToCSV()
	if err != nil {
		return fmt.Errorf("failed to generate CSV: %w", err)
	}

	if err := os.WriteFile(filePath, []byte(csvContent), 0644); err != nil {
		return fmt.Errorf("failed to write CSV file: %w", err)
	}

	return nil
}

func SaveFileQueryResultsAsMarkdown(filename string, queryResults []QueryResultWithQuery, connection string, configDir string) (string, error) {
	// Create sessions directory structure
	sessionDir := filepath.Join(configDir, "sessions", connection)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	mdFilename := fmt.Sprintf("file_results_%s_%s.md", strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)), timestamp)
	fullPath := filepath.Join(sessionDir, mdFilename)

	// Create markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# File Query Results - %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**Connection:** %s\n\n", connection))
	content.WriteString(fmt.Sprintf("**Source File:** %s\n\n", filename))
	content.WriteString(fmt.Sprintf("**Total Queries:** %d\n\n", len(queryResults)))

	// Add each query result
	for i, qr := range queryResults {
		content.WriteString(fmt.Sprintf("## Query %d\n\n", i+1))
		content.WriteString(fmt.Sprintf("**SQL:**\n```sql\n%s\n```\n\n", qr.Query))
		content.WriteString(fmt.Sprintf("**Results:** %d rows\n\n", len(qr.Result.Rows)))

		// Add the markdown table (limited to 20 rows)
		content.WriteString(qr.Result.ToMarkdown(20))
		content.WriteString("\n\n")
	}

	// Write to file
	if err := os.WriteFile(fullPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf("failed to write markdown file: %w", err)
	}

	return fullPath, nil
}

type QueryResultWithQuery struct {
	Result *QueryResult
	Query  string
}
