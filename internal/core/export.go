package core

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sqlterm/internal/i18n"
	"strings"
	"time"
)

func ToMarkdown(result *QueryResult, limit int, i18nMgr *i18n.Manager) string {
	count := 0
	defer result.Close()

	var sb strings.Builder

	// Calculate column widths
	widths := make([]int, len(result.Columns))
	rowsToProcess := make([][]string, 0)
	for i, col := range result.Columns {
		widths[i] = len(col.Name)
	}

	for row := range result.Itor() {
		line := make([]string, len(result.Columns))
		rowsToProcess = append(rowsToProcess, line)
		for i, val := range row {
			if i < len(widths) && len(val.String()) > widths[i] {
				widths[i] = len(val.String())
			}
			line[i] = val.String()
		}
		count++
		if count >= limit {
			break
		}
	}

	if result.Error() != nil {
		sb.WriteString(fmt.Sprint(i18nMgr.Get("query_error"), result.Error()))
		return sb.String()
	}

	// Write header
	sb.WriteString("| ")
	for i, col := range result.Columns {
		sb.WriteString(fmt.Sprintf("%-*s", widths[i], col.Name))
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
				sb.WriteString(fmt.Sprintf("%-*s", widths[i], val))
			}
			if i < len(result.Columns)-1 {
				sb.WriteString(" | ")
			}
		}
		sb.WriteString(" |\n")
	}

	// Add truncation note if limited
	if limit > 0 && count >= limit {
		sb.WriteString(fmt.Sprintf("\n%s\n", i18nMgr.GetWithArgs("markdown_truncation_note", limit)))
	}

	return sb.String()
}

func SaveQueryResultAsMarkdown(result *QueryResult, query string, connection string, resultWriter io.Writer, i18nMgr *i18n.Manager) error {
	// Format the SQL query for better readability
	formatter := NewSQLFormatter()
	formattedQuery := formatter.Format(query)

	// Create markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("%s\n```sql\n%s\n```\n\n", i18nMgr.Get("markdown_query_header"), formattedQuery))

	// Add the markdown table (limited to 20 rows)
	content.WriteString(ToMarkdown(result, 20, i18nMgr))
	content.WriteString("\n\n")

	// Write to file
	if _, err := resultWriter.Write([]byte(content.String())); err != nil {
		return fmt.Errorf(i18nMgr.Get("failed_to_write_markdown"), err)
	}

	return nil
}

// StreamCSVWriter handles streaming CSV writes for large result sets
type StreamCSVWriter struct {
	file   *os.File
	writer *csv.Writer
}

func NewStreamCSVWriter(filePath string) (*StreamCSVWriter, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV file: %w", err)
	}

	writer := csv.NewWriter(file)
	return &StreamCSVWriter{
		file:   file,
		writer: writer,
	}, nil
}

func (w *StreamCSVWriter) WriteHeaders(columns []string) error {
	return w.writer.Write(columns)
}

func (w *StreamCSVWriter) WriteRow(row []Value) error {
	record := make([]string, len(row))
	for i, val := range row {
		record[i] = val.String()
	}
	return w.writer.Write(record)
}

func (w *StreamCSVWriter) Close() error {
	w.writer.Flush()
	if err := w.writer.Error(); err != nil {
		w.file.Close()
		return fmt.Errorf("CSV writer error: %w", err)
	}
	return w.file.Close()
}

func SaveQueryResultAsStreamingCSV(result *QueryResult, filePath string) (int, error) {
	count := 0
	defer result.Close()
	writer, err := NewStreamCSVWriter(filePath)
	if err != nil {
		return count, err
	}
	defer writer.Close()

	// Write headers
	if err := writer.WriteHeaders(result.ColumnNames()); err != nil {
		return count, fmt.Errorf("failed to write CSV headers: %w", err)
	}

	// Write rows one by one
	for row := range result.Itor() {
		if err := writer.WriteRow(row); err != nil {
			return count, fmt.Errorf("failed to write CSV row: %w", err)
		}
		count++
	}

	if result.Error() != nil {
		return count, fmt.Errorf("failed to fetch data: %w", err)
	}

	return count, nil
}

// GenerateNumberedCSVPath creates a numbered CSV filename for multiple queries
func GenerateNumberedCSVPath(baseFilePath string, queryIndex int) string {
	dir := filepath.Dir(baseFilePath)
	filename := filepath.Base(baseFilePath)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	if dir == "." {
		return fmt.Sprintf("%s-%d%s", nameWithoutExt, queryIndex, ext)
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d%s", nameWithoutExt, queryIndex, ext))
}

func SaveFileQueryResultsAsMarkdown(filename string, queryResults []QueryResultWithQuery, connection string, configDir string, i18nMgr *i18n.Manager) (string, error) {
	// Create sessions directory structure
	sessionDir := filepath.Join(configDir, "sessions", connection)
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf(i18nMgr.Get("failed_to_create_session_dir"), err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	mdFilename := fmt.Sprintf("file_results_%s_%s.md", strings.TrimSuffix(filepath.Base(filename), filepath.Ext(filename)), timestamp)
	fullPath := filepath.Join(sessionDir, mdFilename)

	// Create markdown content
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s - %s\n\n", i18nMgr.Get("file_query_results_header"), time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**%s:** %s\n\n", i18nMgr.Get("connection_header"), connection))
	content.WriteString(fmt.Sprintf("**%s:** %s\n\n", i18nMgr.Get("source_file_header"), filename))
	content.WriteString(fmt.Sprintf("**%s:** %d\n\n", i18nMgr.Get("total_queries_header"), len(queryResults)))

	// Add each query result
	for i, qr := range queryResults {
		content.WriteString(fmt.Sprintf("## %s %d\n\n", i18nMgr.Get("query_header"), i+1))
		content.WriteString(fmt.Sprintf("**SQL:**\n```sql\n%s\n```\n\n", qr.Query))

		// Add the markdown table (limited to 20 rows)
		content.WriteString(ToMarkdown(qr.Result, 20, i18nMgr))
		content.WriteString("\n\n")
	}

	// Write to file
	if err := os.WriteFile(fullPath, []byte(content.String()), 0644); err != nil {
		return "", fmt.Errorf(i18nMgr.Get("failed_to_write_markdown"), err)
	}

	return fullPath, nil
}

type QueryResultWithQuery struct {
	Result *QueryResult
	Query  string
}
