package config

import "testing"

func TestParseDatabaseType(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected DatabaseType
		hasError bool
	}{
		{
			name:     "MySQL lowercase",
			input:    "mysql",
			expected: MySQL,
			hasError: false,
		},
		{
			name:     "MySQL uppercase",
			input:    "MYSQL",
			expected: MySQL,
			hasError: false,
		},
		{
			name:     "PostgreSQL lowercase",
			input:    "postgres",
			expected: PostgreSQL,
			hasError: false,
		},
		{
			name:     "PostgreSQL alternative",
			input:    "postgresql",
			expected: PostgreSQL,
			hasError: false,
		},
		{
			name:     "SQLite",
			input:    "sqlite",
			expected: SQLite,
			hasError: false,
		},
		{
			name:     "SQLite3",
			input:    "sqlite3",
			expected: SQLite,
			hasError: false,
		},
		{
			name:     "Invalid type",
			input:    "mongodb",
			expected: DatabaseType(0),
			hasError: true,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: DatabaseType(0),
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := ParseDatabaseType(tc.input)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for input '%s', but got none", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input '%s': %v", tc.input, err)
				}

				if result != tc.expected {
					t.Errorf("Expected %v, got %v for input '%s'", tc.expected, result, tc.input)
				}
			}
		})
	}
}

func TestGetDefaultPort(t *testing.T) {
	testCases := []struct {
		name     string
		dbType   DatabaseType
		expected int
	}{
		{
			name:     "MySQL default port",
			dbType:   MySQL,
			expected: 3306,
		},
		{
			name:     "PostgreSQL default port",
			dbType:   PostgreSQL,
			expected: 5432,
		},
		{
			name:     "SQLite default port",
			dbType:   SQLite,
			expected: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GetDefaultPort(tc.dbType)
			if result != tc.expected {
				t.Errorf("Expected port %d, got %d for %v", tc.expected, result, tc.dbType)
			}
		})
	}
}

func TestDatabaseType_String(t *testing.T) {
	testCases := []struct {
		name     string
		dbType   DatabaseType
		expected string
	}{
		{
			name:     "MySQL string representation",
			dbType:   MySQL,
			expected: "mysql",
		},
		{
			name:     "PostgreSQL string representation",
			dbType:   PostgreSQL,
			expected: "postgres",
		},
		{
			name:     "SQLite string representation",
			dbType:   SQLite,
			expected: "sqlite",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.dbType.String()
			if result != tc.expected {
				t.Errorf("Expected string '%s', got '%s' for %v", tc.expected, result, tc.dbType)
			}
		})
	}
}
