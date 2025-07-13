package core

import (
	"regexp"
	"strings"
)

// SQLFormatter formats SQL queries for better readability
type SQLFormatter struct {
	indentSize int
}

// NewSQLFormatter creates a new SQL formatter
func NewSQLFormatter() *SQLFormatter {
	return &SQLFormatter{
		indentSize: 4, // 4 spaces for indentation
	}
}

// Format formats a SQL query for better readability
func (f *SQLFormatter) Format(sql string) string {
	if strings.TrimSpace(sql) == "" {
		return sql
	}

	// Clean up the input
	sql = strings.TrimSpace(sql)
	sql = f.normalizeWhitespace(sql)

	// Check if it's likely a SQL query
	if !f.isSQLQuery(sql) {
		return sql
	}

	// Apply formatting
	formatted := f.formatSQL(sql)
	return formatted
}

// normalizeWhitespace cleans up excessive whitespace
func (f *SQLFormatter) normalizeWhitespace(sql string) string {
	// Replace multiple whitespace with single space
	re := regexp.MustCompile(`\s+`)
	sql = re.ReplaceAllString(sql, " ")
	
	// Clean up whitespace around commas and parentheses
	sql = regexp.MustCompile(`\s*,\s*`).ReplaceAllString(sql, ", ")
	sql = regexp.MustCompile(`\s*\(\s*`).ReplaceAllString(sql, " (")
	sql = regexp.MustCompile(`\s*\)\s*`).ReplaceAllString(sql, ") ")
	
	return strings.TrimSpace(sql)
}

// isSQLQuery checks if the string looks like a SQL query
func (f *SQLFormatter) isSQLQuery(sql string) bool {
	sqlUpper := strings.ToUpper(sql)
	
	// Common SQL keywords that indicate this is likely a SQL query
	sqlKeywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "ALTER",
		"WITH", "MERGE", "UPSERT", "EXPLAIN", "SHOW", "DESCRIBE", "DESC",
	}
	
	for _, keyword := range sqlKeywords {
		if strings.HasPrefix(sqlUpper, keyword+" ") || strings.HasPrefix(sqlUpper, keyword+"\t") || sqlUpper == keyword {
			return true
		}
	}
	
	return false
}

// formatSQL applies SQL formatting rules
func (f *SQLFormatter) formatSQL(sql string) string {
	// Keywords that should start new lines
	majorKeywords := []string{
		"SELECT", "FROM", "WHERE", "GROUP BY", "HAVING", "ORDER BY", "LIMIT", "OFFSET",
		"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
		"CREATE", "DROP", "ALTER", "WITH", "UNION", "UNION ALL", "INTERSECT", "EXCEPT",
		"CASE", "WHEN", "THEN", "ELSE", "END",
	}
	
	// Keywords that should be indented
	joinKeywords := []string{
		"INNER JOIN", "LEFT JOIN", "RIGHT JOIN", "FULL JOIN", "FULL OUTER JOIN",
		"LEFT OUTER JOIN", "RIGHT OUTER JOIN", "CROSS JOIN", "JOIN",
	}
	
	// Keywords that should be further indented
	subKeywords := []string{
		"ON", "AND", "OR",
	}

	lines := []string{}
	currentLine := ""
	words := strings.Fields(sql)
	
	i := 0
	for i < len(words) {
		word := words[i]
		wordUpper := strings.ToUpper(word)
		
		// Check for multi-word keywords first
		matched := false
		
		// Check for two-word keywords
		if i < len(words)-1 {
			twoWord := wordUpper + " " + strings.ToUpper(words[i+1])
			if f.contains(append(majorKeywords, joinKeywords...), twoWord) {
				// Finish current line if it has content
				if strings.TrimSpace(currentLine) != "" {
					lines = append(lines, strings.TrimSpace(currentLine))
				}
				
				// Start new line with appropriate indentation
				if f.contains(joinKeywords, twoWord) {
					currentLine = f.indent(1) + twoWord
				} else {
					currentLine = twoWord
				}
				i += 2
				matched = true
			}
		}
		
		if !matched {
			// Check for single-word keywords
			if f.contains(majorKeywords, wordUpper) {
				// Finish current line if it has content
				if strings.TrimSpace(currentLine) != "" {
					lines = append(lines, strings.TrimSpace(currentLine))
				}
				currentLine = wordUpper
				i++
			} else if f.contains(joinKeywords, wordUpper) {
				// Finish current line if it has content
				if strings.TrimSpace(currentLine) != "" {
					lines = append(lines, strings.TrimSpace(currentLine))
				}
				currentLine = f.indent(1) + wordUpper
				i++
			} else if f.contains(subKeywords, wordUpper) && strings.TrimSpace(currentLine) != "" {
				// Add to current line but on new line with extra indent for readability
				lines = append(lines, strings.TrimSpace(currentLine))
				currentLine = f.indent(2) + wordUpper
				i++
			} else {
				// Regular word, add to current line
				if currentLine == "" {
					currentLine = word
				} else {
					currentLine += " " + word
				}
				i++
			}
		}
	}
	
	// Add the last line
	if strings.TrimSpace(currentLine) != "" {
		lines = append(lines, strings.TrimSpace(currentLine))
	}
	
	// Post-process to handle SELECT columns nicely
	result := f.formatSelectColumns(lines)
	
	// Add semicolon if missing
	if len(result) > 0 && !strings.HasSuffix(result[len(result)-1], ";") {
		result[len(result)-1] += ";"
	}
	
	return strings.Join(result, "\n")
}

// formatSelectColumns formats SELECT column lists for better readability
func (f *SQLFormatter) formatSelectColumns(lines []string) []string {
	result := []string{}
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(strings.ToUpper(line), "SELECT ") {
			// Extract the SELECT part and columns
			selectPart := "SELECT"
			columns := strings.TrimSpace(line[6:]) // Remove "SELECT"
			
			if columns == "*" {
				result = append(result, selectPart+" *")
			} else if columns != "" {
				// Split columns by comma and format nicely
				columnParts := strings.Split(columns, ",")
				if len(columnParts) > 1 {
					result = append(result, selectPart)
					for j, col := range columnParts {
						col = strings.TrimSpace(col)
						if j == len(columnParts)-1 {
							// Last column, no comma
							result = append(result, f.indent(1)+col)
						} else {
							result = append(result, f.indent(1)+col+",")
						}
					}
				} else {
					result = append(result, line)
				}
			} else {
				result = append(result, selectPart)
			}
		} else {
			result = append(result, line)
		}
	}
	
	return result
}

// indent creates indentation string
func (f *SQLFormatter) indent(level int) string {
	return strings.Repeat(" ", level*f.indentSize)
}

// contains checks if slice contains string (case-sensitive)
func (f *SQLFormatter) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// FormatSQLInMarkdown finds and formats SQL code blocks in markdown
func FormatSQLInMarkdown(markdown string) string {
	// Find SQL code blocks with ```sql
	sqlBlockRegex := regexp.MustCompile("(?s)```sql\\s*\\n(.*?)\\n```")
	formatter := NewSQLFormatter()
	
	return sqlBlockRegex.ReplaceAllStringFunc(markdown, func(match string) string {
		// Extract SQL content
		content := sqlBlockRegex.FindStringSubmatch(match)
		if len(content) < 2 {
			return match
		}
		
		sqlContent := content[1]
		formattedSQL := formatter.Format(sqlContent)
		
		return "```sql\n" + formattedSQL + "\n```"
	})
}