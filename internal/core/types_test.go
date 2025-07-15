package core

import (
	"testing"
)

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

func TestStringValue(t *testing.T) {
	testCases := []struct {
		name     string
		value    StringValue
		isNull   bool
		expected string
	}{
		{
			name:     "Non-null string",
			value:    StringValue{Value: "test"},
			isNull:   false,
			expected: "test",
		},
		{
			name:     "Null string",
			value:    StringValue{Value: "", Null: true},
			isNull:   true,
			expected: "",
		},
		{
			name:     "Empty but valid string",
			value:    StringValue{Value: ""},
			isNull:   false,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value.IsNull() != tc.isNull {
				t.Errorf("Expected IsNull() to return %v, got %v", tc.isNull, tc.value.IsNull())
			}
			
			if tc.value.String() != tc.expected {
				t.Errorf("Expected String() to return '%s', got '%s'", tc.expected, tc.value.String())
			}
		})
	}
}

func TestIntValue(t *testing.T) {
	testCases := []struct {
		name     string
		value    IntValue
		isNull   bool
		expected string
	}{
		{
			name:     "Non-null int",
			value:    IntValue{Value: 42},
			isNull:   false,
			expected: "42",
		},
		{
			name:     "Null int",
			value:    IntValue{Value: 0, Null: true},
			isNull:   true,
			expected: "",
		},
		{
			name:     "Zero but valid int",
			value:    IntValue{Value: 0},
			isNull:   false,
			expected: "0",
		},
		{
			name:     "Negative int",
			value:    IntValue{Value: -123},
			isNull:   false,
			expected: "-123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value.IsNull() != tc.isNull {
				t.Errorf("Expected IsNull() to return %v, got %v", tc.isNull, tc.value.IsNull())
			}
			
			if tc.value.String() != tc.expected {
				t.Errorf("Expected String() to return '%s', got '%s'", tc.expected, tc.value.String())
			}
		})
	}
}

func TestFloatValue(t *testing.T) {
	testCases := []struct {
		name     string
		value    FloatValue
		isNull   bool
		expected string
	}{
		{
			name:     "Non-null float",
			value:    FloatValue{Value: 3.14159},
			isNull:   false,
			expected: "3.14159",
		},
		{
			name:     "Null float",
			value:    FloatValue{Value: 0.0, Null: true},
			isNull:   true,
			expected: "",
		},
		{
			name:     "Zero but valid float",
			value:    FloatValue{Value: 0.0},
			isNull:   false,
			expected: "0",
		},
		{
			name:     "Negative float",
			value:    FloatValue{Value: -1.5},
			isNull:   false,
			expected: "-1.5",
		},
		{
			name:     "Large float",
			value:    FloatValue{Value: 1234567.89},
			isNull:   false,
			expected: "1.23456789e+06",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value.IsNull() != tc.isNull {
				t.Errorf("Expected IsNull() to return %v, got %v", tc.isNull, tc.value.IsNull())
			}
			
			if tc.value.String() != tc.expected {
				t.Errorf("Expected String() to return '%s', got '%s'", tc.expected, tc.value.String())
			}
		})
	}
}

func TestBoolValue(t *testing.T) {
	testCases := []struct {
		name     string
		value    BoolValue
		isNull   bool
		expected string
	}{
		{
			name:     "True value",
			value:    BoolValue{Value: true},
			isNull:   false,
			expected: "true",
		},
		{
			name:     "False value",
			value:    BoolValue{Value: false},
			isNull:   false,
			expected: "false",
		},
		{
			name:     "Null bool",
			value:    BoolValue{Value: false, Null: true},
			isNull:   true,
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value.IsNull() != tc.isNull {
				t.Errorf("Expected IsNull() to return %v, got %v", tc.isNull, tc.value.IsNull())
			}
			
			if tc.value.String() != tc.expected {
				t.Errorf("Expected String() to return '%s', got '%s'", tc.expected, tc.value.String())
			}
		})
	}
}

func TestNullValue(t *testing.T) {
	nullValue := NullValue{}
	
	if !nullValue.IsNull() {
		t.Error("NullValue.IsNull() should always return true")
	}
	
	if nullValue.String() != "" {
		t.Errorf("NullValue.String() should return empty string, got '%s'", nullValue.String())
	}
}

func TestGenerateNumberedCSVPath(t *testing.T) {
	testCases := []struct {
		name         string
		basePath     string
		queryIndex   int
		expected     string
	}{
		{
			name:       "First query",
			basePath:   "/path/to/results.csv",
			queryIndex: 1,
			expected:   "/path/to/results-1.csv",
		},
		{
			name:       "Multiple queries",
			basePath:   "/path/to/export.csv",
			queryIndex: 5,
			expected:   "/path/to/export-5.csv",
		},
		{
			name:       "Path without extension",
			basePath:   "/path/to/file",
			queryIndex: 2,
			expected:   "/path/to/file-2",
		},
		{
			name:       "Complex path",
			basePath:   "/complex/path.with.dots/file.csv",
			queryIndex: 10,
			expected:   "/complex/path.with.dots/file-10.csv",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := GenerateNumberedCSVPath(tc.basePath, tc.queryIndex)
			if result != tc.expected {
				t.Errorf("Expected path '%s', got '%s'", tc.expected, result)
			}
		})
	}
}