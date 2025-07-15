package core

import (
	"strings"
	"testing"
)

func TestNewSQLFormatter(t *testing.T) {
	formatter := NewSQLFormatter()

	if formatter == nil {
		t.Fatal("NewSQLFormatter() returned nil")
	}

	// Check that the formatter has reasonable default values
	if formatter.indentSize <= 0 {
		t.Error("Formatter should have positive indent size")
	}
}

func TestSQLFormatter_isSQLQuery(t *testing.T) {
	formatter := NewSQLFormatter()

	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "SELECT query",
			input:    "SELECT * FROM users",
			expected: true,
		},
		{
			name:     "INSERT query",
			input:    "INSERT INTO users (name) VALUES ('John')",
			expected: true,
		},
		{
			name:     "UPDATE query",
			input:    "UPDATE users SET name = 'Jane' WHERE id = 1",
			expected: true,
		},
		{
			name:     "DELETE query",
			input:    "DELETE FROM users WHERE id = 1",
			expected: true,
		},
		{
			name:     "CREATE query",
			input:    "CREATE TABLE users (id INT, name VARCHAR(255))",
			expected: true,
		},
		{
			name:     "DROP query",
			input:    "DROP TABLE users",
			expected: true,
		},
		{
			name:     "WITH query",
			input:    "WITH cte AS (SELECT * FROM users) SELECT * FROM cte",
			expected: true,
		},
		{
			name:     "Non-SQL text",
			input:    "This is just regular text",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "Whitespace only",
			input:    "   \n\t  ",
			expected: false,
		},
		{
			name:     "SELECT in middle of text",
			input:    "Please SELECT some data from the table",
			expected: false,
		},
		{
			name:     "Case insensitive",
			input:    "select * from users",
			expected: true,
		},
		{
			name:     "Leading whitespace",
			input:    "  \n  SELECT * FROM users",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatter.isSQLQuery(tc.input)
			if result != tc.expected {
				t.Errorf("Expected isSQLQuery('%s') to return %v, got %v", tc.input, tc.expected, result)
			}
		})
	}
}

func TestSQLFormatter_normalizeWhitespace(t *testing.T) {
	formatter := NewSQLFormatter()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Multiple spaces",
			input:    "SELECT     *     FROM     users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Mixed whitespace",
			input:    "SELECT\t\n *  \r\n FROM\t users",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Leading and trailing whitespace",
			input:    "   SELECT * FROM users   ",
			expected: "SELECT * FROM users",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only whitespace",
			input:    "   \n\t  ",
			expected: "",
		},
		{
			name:     "No extra whitespace",
			input:    "SELECT * FROM users",
			expected: "SELECT * FROM users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatter.normalizeWhitespace(tc.input)
			if result != tc.expected {
				t.Errorf("Expected normalizeWhitespace('%s') to return '%s', got '%s'", tc.input, tc.expected, result)
			}
		})
	}
}

func TestSQLFormatter_Format(t *testing.T) {
	formatter := NewSQLFormatter()

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple SELECT",
			input:    "select * from users",
			expected: "SELECT *\nFROM users;",
		},
		{
			name:     "Complex SELECT with WHERE",
			input:    "select name, email from users where age > 18",
			expected: "SELECT\n    name,\n    email\nFROM users\nWHERE age > 18;",
		},
		{
			name:     "INSERT statement",
			input:    "insert into users (name, email) values ('John', 'john@example.com')",
			expected: "INSERT\nINTO users (name, email)\nVALUES ('John', 'john@example.com');",
		},
		{
			name:     "UPDATE statement",
			input:    "update users set name = 'Jane' where id = 1",
			expected: "UPDATE users\nSET name = 'Jane'\nWHERE id = 1;",
		},
		{
			name:     "DELETE statement",
			input:    "delete from users where age < 18",
			expected: "DELETE\nFROM users\nWHERE age < 18;",
		},
		{
			name:     "CREATE TABLE",
			input:    "create table users (id int primary key, name varchar(255))",
			expected: "CREATE table users (id int primary key, name varchar (255) );",
		},
		{
			name:     "Non-SQL text",
			input:    "This is just regular text",
			expected: "This is just regular text",
		},
		{
			name:     "Mixed case keywords",
			input:    "Select Name, Email From Users Where Age > 18",
			expected: "SELECT\n    Name,\n    Email\nFROM Users\nWHERE Age > 18;",
		},
		{
			name:     "With extra whitespace",
			input:    "   select    *    from    users   ",
			expected: "SELECT *\nFROM users;",
		},
		{
			name:     "JOIN query",
			input:    "select u.name, p.title from users u join posts p on u.id = p.user_id",
			expected: "SELECT\n    u.name,\n    p.title\nFROM users u\nJOIN posts p\nON u.id = p.user_id;",
		},
		{
			name:     "Subquery",
			input:    "select * from (select name from users) as subquery",
			expected: "SELECT *\nFROM (select name\nFROM users)\nAS subquery;",
		},
		{
			name:     "WITH clause",
			input:    "with active_users as (select * from users where active = true) select * from active_users",
			expected: "WITH active_users\nAS (select *\nFROM users\nWHERE active = true)\nSELECT *\nFROM active_users;",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatter.Format(tc.input)
			if result != tc.expected {
				t.Errorf("Expected Format('%s') to return '%s', got '%s'", tc.input, tc.expected, result)
			}
		})
	}
}

func TestFormatSQLInMarkdown(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "SQL code block",
			input:    "```sql\nselect * from users\n```",
			expected: "```sql\nSELECT *\nFROM users;\n```",
		},
		{
			name:     "Multiple SQL blocks",
			input:    "```sql\nselect * from users\n```\n\nSome text\n\n```sql\ninsert into users (name) values ('John')\n```",
			expected: "```sql\nSELECT *\nFROM users;\n```\n\nSome text\n\n```sql\nINSERT\nINTO users (name)\nVALUES ('John');\n```",
		},
		{
			name:     "SQL block with language specifier",
			input:    "```SQL\nselect * from users\n```",
			expected: "```SQL\nselect * from users\n```",
		},
		{
			name:     "Non-SQL code block",
			input:    "```python\nprint('hello')\n```",
			expected: "```python\nprint('hello')\n```",
		},
		{
			name:     "Mixed content",
			input:    "# Query Results\n\n```sql\nselect name from users\n```\n\nThis is the result.",
			expected: "# Query Results\n\n```sql\nSELECT name\nFROM users;\n```\n\nThis is the result.",
		},
		{
			name:     "No code blocks",
			input:    "Just regular markdown text",
			expected: "Just regular markdown text",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Inline code",
			input:    "Use `select * from users` to get all users",
			expected: "Use `select * from users` to get all users",
		},
		{
			name:     "Malformed code block",
			input:    "```sql\nselect * from users",
			expected: "```sql\nselect * from users",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FormatSQLInMarkdown(tc.input)
			if result != tc.expected {
				t.Errorf("Expected FormatSQLInMarkdown('%s') to return '%s', got '%s'", tc.input, tc.expected, result)
			}
		})
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

// Benchmark tests for performance
func BenchmarkSQLFormatter_Format(b *testing.B) {
	formatter := NewSQLFormatter()
	testSQL := "select u.name, u.email, p.title from users u join posts p on u.id = p.user_id where u.active = true and p.published = true order by u.name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatter.Format(testSQL)
	}
}

func BenchmarkFormatSQLInMarkdown(b *testing.B) {
	markdown := `# Query Results

Here are the results:

` + "```sql\nselect u.name, u.email, p.title from users u join posts p on u.id = p.user_id where u.active = true\n```" + `

And another query:

` + "```sql\ninsert into users (name, email) values ('John', 'john@example.com')\n```"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		FormatSQLInMarkdown(markdown)
	}
}

func TestSQLFormatter_ComplexQueries(t *testing.T) {
	formatter := NewSQLFormatter()

	testCases := []struct {
		name     string
		input    string
		contains []string // Check that these keywords are properly capitalized
	}{
		{
			name: "Complex JOIN query",
			input: `select u.name, u.email, p.title, c.name as category
					from users u
					inner join posts p on u.id = p.user_id
					left join categories c on p.category_id = c.id
					where u.active = true
					order by u.name`,
			contains: []string{"SELECT", "FROM", "INNER JOIN", "LEFT JOIN", "ON", "WHERE", "ORDER BY"},
		},
		{
			name: "Window functions",
			input: `select name, salary,
					row_number() over (partition by department order by salary desc) as rank
					from employees`,
			contains: []string{"SELECT", "FROM", "OVER", "ORDER BY"},
		},
		{
			name: "CTE query",
			input: `with department_avg as (
					select department, avg(salary) as avg_salary
					from employees
					group by department
					)
					select e.name, e.salary, d.avg_salary
					from employees e
					join department_avg d on e.department = d.department`,
			contains: []string{"WITH", "SELECT", "FROM", "GROUP BY", "JOIN", "ON"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := formatter.Format(tc.input)

			// Check that all expected keywords are present and capitalized
			for _, keyword := range tc.contains {
				if !strings.Contains(result, keyword) {
					t.Errorf("Expected formatted SQL to contain '%s', but it didn't. Result: %s", keyword, result)
				}
			}

			// Check that the result is different from input (should be formatted)
			if result == tc.input {
				t.Errorf("Expected formatted SQL to be different from input, but they were the same")
			}
		})
	}
}
