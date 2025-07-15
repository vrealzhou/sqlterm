package conversation

import (
	"os"
	"path/filepath"
	"testing"

	"sqlterm/internal/core"
)

func TestAutoCompleter_NewAutoCompleter(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	if ac == nil {
		t.Fatal("NewAutoCompleter() returned nil")
	}

	if ac.app != app {
		t.Error("AutoCompleter should have reference to app")
	}
}

func TestAutoCompleter_getCommands(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	commands := ac.getCommands()

	if len(commands) == 0 {
		t.Error("getCommands() should return at least one command")
	}

	// Check for expected commands
	expectedCommands := []string{"/help", "/quit", "/connect", "/status", "/exec"}
	commandStrings := make([]string, len(commands))
	for i, cmd := range commands {
		commandStrings[i] = string(cmd)
	}

	for _, expected := range expectedCommands {
		found := false
		for _, cmd := range commandStrings {
			if cmd == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected command '%s' not found in commands", expected)
		}
	}
}

func TestAutoCompleter_getCommandCandidates(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name     string
		partial  string
		expected []string
	}{
		{
			name:     "Help command prefix",
			partial:  "/h",
			expected: []string{"elp"},
		},
		{
			name:     "Connect command prefix",
			partial:  "/con",
			expected: []string{"nect", "fig"},
		},
		{
			name:     "Multiple matches",
			partial:  "/",
			expected: []string{"help", "quit", "exit", "connect", "list-connections", "tables", "describe", "status", "exec", "config", "prompts", "clear-conversation"},
		},
		{
			name:     "No matches",
			partial:  "/xyz",
			expected: []string{},
		},
		{
			name:     "Exact match",
			partial:  "/help",
			expected: []string{""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := ac.getCommandCandidates(tc.partial)

			if len(candidates) != len(tc.expected) {
				t.Errorf("Expected %d candidates, got %d", len(tc.expected), len(candidates))
				return
			}

			for i, expected := range tc.expected {
				if candidates[i] != expected {
					t.Errorf("Expected candidate '%s', got '%s'", expected, candidates[i])
				}
			}
		})
	}
}

func TestAutoCompleter_getConfigCandidates(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name     string
		words    []string
		line     string
		expected []string
	}{
		{
			name:     "Main config sections",
			words:    []string{"/config", "a"},
			line:     "/config a",
			expected: []string{"i"},
		},
		{
			name:     "AI subcommands",
			words:    []string{"/config", "ai", "p"},
			line:     "/config ai p",
			expected: []string{"rovider"},
		},
		{
			name:     "AI provider candidates",
			words:    []string{"/config", "ai", "provider", "o"},
			line:     "/config ai provider o",
			expected: []string{"penrouter", "llama"},
		},
		{
			name:     "Language candidates",
			words:    []string{"/config", "language", "e"},
			line:     "/config language e",
			expected: []string{"n_au"},
		},
		{
			name:     "No matches",
			words:    []string{"/config", "invalid"},
			line:     "/config invalid",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := ac.getConfigCandidates(tc.words, tc.line)

			if len(candidates) != len(tc.expected) {
				t.Errorf("Expected %d candidates, got %d", len(tc.expected), len(candidates))
				return
			}

			for i, expected := range tc.expected {
				if candidates[i] != expected {
					t.Errorf("Expected candidate '%s', got '%s'", expected, candidates[i])
				}
			}
		})
	}
}

func TestAutoCompleter_getTableCandidates(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	// Test without connection
	candidates := ac.getTableCandidates([]string{"/describe", "u"}, "/describe u")
	if len(candidates) != 0 {
		t.Error("Should return no candidates without connection")
	}

	// Test with connection
	mockConn := &mockConnection{
		tables:    []string{"users", "user_profiles", "posts"},
		connected: true,
		dbType:    core.PostgreSQL,
		name:      "test-db",
	}

	config := &core.ConnectionConfig{
		Name: "test-db",
	}

	// Disable AI manager to avoid vector database initialization issues
	app.aiManager = nil

	app.SetConnection(mockConn, config)

	testCases := []struct {
		name     string
		words    []string
		line     string
		expected []string
	}{
		{
			name:     "Partial table name",
			words:    []string{"/describe", "u"},
			line:     "/describe u",
			expected: []string{"sers", "ser_profiles"},
		},
		{
			name:     "No matches",
			words:    []string{"/describe", "xyz"},
			line:     "/describe xyz",
			expected: []string{},
		},
		{
			name:     "Empty current word",
			words:    []string{"/describe", ""},
			line:     "/describe ",
			expected: []string{"users", "user_profiles", "posts"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := ac.getTableCandidates(tc.words, tc.line)

			if len(candidates) != len(tc.expected) {
				t.Errorf("Expected %d candidates, got %d", len(tc.expected), len(candidates))
				return
			}

			for i, expected := range tc.expected {
				if candidates[i] != expected {
					t.Errorf("Expected candidate '%s', got '%s'", expected, candidates[i])
				}
			}
		})
	}
}

func TestAutoCompleter_getFileCandidates(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	// Create temporary directory with test files
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	// Create test files
	testFiles := []string{
		"query1.sql",
		"query2.sql",
		"data.csv",
		"subdir/nested.sql",
	}

	for _, file := range testFiles {
		dir := filepath.Dir(file)
		if dir != "." {
			os.MkdirAll(dir, 0755)
		}
		os.WriteFile(file, []byte("test content"), 0644)
	}

	testCases := []struct {
		name     string
		line     string
		expected []string
	}{
		{
			name:     "Empty path",
			line:     "@",
			expected: []string{"query1.sql", "query2.sql", "subdir/", "subdir/nested.sql"},
		},
		{
			name:     "Partial filename",
			line:     "@q",
			expected: []string{"uery1.sql", "uery2.sql"},
		},
		{
			name:     "Directory path",
			line:     "@subdir/",
			expected: []string{"nested.sql"},
		},
		{
			name:     "No matches",
			line:     "@nonexistent",
			expected: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := ac.getFileCandidates(tc.line)

			if len(candidates) != len(tc.expected) {
				t.Errorf("Expected %d candidates, got %d", len(tc.expected), len(candidates))
				return
			}

			for i, expected := range tc.expected {
				if candidates[i] != expected {
					t.Errorf("Expected candidate '%s', got '%s'", expected, candidates[i])
				}
			}
		})
	}
}

func TestAutoCompleter_getCSVCandidates(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	// Create temporary directory with test files
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	// Create test files
	testFiles := []string{
		"output.csv",
		"data.csv",
		"results.txt",
		"export.xlsx",
	}

	for _, file := range testFiles {
		os.WriteFile(file, []byte("test content"), 0644)
	}

	testCases := []struct {
		name     string
		words    []string
		line     string
		expected []string
	}{
		{
			name:     "CSV files with partial name",
			words:    []string{"SELECT", "*", "FROM", "users", ">", "o"},
			line:     "SELECT * FROM users > o",
			expected: []string{"utput.csv", ".csv"},
		},
		{
			name:     "All files with empty name",
			words:    []string{"SELECT", "*", "FROM", "users", ">", ""},
			line:     "SELECT * FROM users > ",
			expected: []string{"data.csv", "export.xlsx", "output.csv", "results.txt"},
		},
		{
			name:     "No matches",
			words:    []string{"SELECT", "*", "FROM", "users", ">", "xyz"},
			line:     "SELECT * FROM users > xyz",
			expected: []string{".csv"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			candidates := ac.getCSVCandidates(tc.words, tc.line)

			if len(candidates) != len(tc.expected) {
				t.Errorf("Expected %d candidates, got %d", len(tc.expected), len(candidates))
				return
			}

			for i, expected := range tc.expected {
				if candidates[i] != expected {
					t.Errorf("Expected candidate '%s', got '%s'", expected, candidates[i])
				}
			}
		})
	}
}

func TestAutoCompleter_findCommonPrefix(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name       string
		candidates []string
		expected   string
	}{
		{
			name:       "Common prefix",
			candidates: []string{"hello", "help", "hero"},
			expected:   "he",
		},
		{
			name:       "No common prefix",
			candidates: []string{"apple", "banana", "cherry"},
			expected:   "",
		},
		{
			name:       "Single candidate",
			candidates: []string{"hello"},
			expected:   "hello",
		},
		{
			name:       "Empty candidates",
			candidates: []string{},
			expected:   "",
		},
		{
			name:       "Identical candidates",
			candidates: []string{"hello", "hello", "hello"},
			expected:   "hello",
		},
		{
			name:       "One shorter candidate",
			candidates: []string{"hello", "help", "he"},
			expected:   "he",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ac.findCommonPrefix(tc.candidates)

			if result != tc.expected {
				t.Errorf("Expected common prefix '%s', got '%s'", tc.expected, result)
			}
		})
	}
}

func TestAutoCompleter_processCompletions(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name          string
		candidates    []string
		typedLength   int
		expectedLen   int
		expectedFirst string
	}{
		{
			name:          "Single candidate",
			candidates:    []string{"hello"},
			typedLength:   2,
			expectedLen:   1,
			expectedFirst: "hello",
		},
		{
			name:          "Multiple candidates with common prefix",
			candidates:    []string{"hello", "help", "hero"},
			typedLength:   2,
			expectedLen:   1,
			expectedFirst: "he",
		},
		{
			name:          "Multiple candidates no common prefix",
			candidates:    []string{"apple", "banana", "cherry"},
			typedLength:   1,
			expectedLen:   3,
			expectedFirst: "apple",
		},
		{
			name:          "No candidates",
			candidates:    []string{},
			typedLength:   0,
			expectedLen:   0,
			expectedFirst: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ac.processCompletions(tc.candidates, tc.typedLength)

			if len(result) != tc.expectedLen {
				t.Errorf("Expected %d completions, got %d", tc.expectedLen, len(result))
				return
			}

			if tc.expectedLen > 0 {
				if string(result[0]) != tc.expectedFirst {
					t.Errorf("Expected first completion '%s', got '%s'", tc.expectedFirst, string(result[0]))
				}
			}
		})
	}
}

func TestAutoCompleter_getCompletionLength(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name     string
		line     string
		expected int
	}{
		{
			name:     "File completion",
			line:     "@query.sql",
			expected: 9,
		},
		{
			name:     "CSV completion",
			line:     "SELECT * FROM users > output.csv",
			expected: 10,
		},
		{
			name:     "Command completion",
			line:     "/connect test-db",
			expected: 7,
		},
		{
			name:     "Empty line",
			line:     "",
			expected: 0,
		},
		{
			name:     "Single word",
			line:     "hello",
			expected: 5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ac.getCompletionLength(tc.line)

			if result != tc.expected {
				t.Errorf("Expected completion length %d, got %d", tc.expected, result)
			}
		})
	}
}

func TestAutoCompleter_shouldSkipDirectory(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name     string
		dirName  string
		expected bool
	}{
		{
			name:     "Node modules",
			dirName:  "node_modules",
			expected: true,
		},
		{
			name:     "Git directory",
			dirName:  ".git",
			expected: true,
		},
		{
			name:     "Regular directory",
			dirName:  "queries",
			expected: false,
		},
		{
			name:     "Build directory",
			dirName:  "build",
			expected: true,
		},
		{
			name:     "Vendor directory",
			dirName:  "vendor",
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := ac.shouldSkipDirectory(tc.dirName)

			if result != tc.expected {
				t.Errorf("Expected shouldSkipDirectory('%s') to return %v, got %v", tc.dirName, tc.expected, result)
			}
		})
	}
}

func TestAutoCompleter_getAvailableModels(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	// Test with no AI manager
	models := ac.getAvailableModels()
	if models == nil {
		t.Error("Should return empty slice when no AI manager is available")
	}

	// Test with AI manager would require setting up a proper AI manager
	// which is complex for unit tests. The method is tested indirectly
	// through the config candidates test.
}

// Integration test for the main Do method
func TestAutoCompleter_Do(t *testing.T) {
	app := createTestApp(t)
	ac := NewAutoCompleter(app)

	testCases := []struct {
		name        string
		line        string
		pos         int
		expectCount int
	}{
		{
			name:        "Empty line",
			line:        "",
			pos:         0,
			expectCount: 12, // Number of commands
		},
		{
			name:        "Command completion",
			line:        "/h",
			pos:         2,
			expectCount: 1,
		},
		{
			name:        "Config completion",
			line:        "/config a",
			pos:         9,
			expectCount: 1,
		},
		{
			name:        "No completion context",
			line:        "SELECT * FROM users",
			pos:         19,
			expectCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			lineRunes := []rune(tc.line)
			newLine, length := ac.Do(lineRunes, tc.pos)

			if len(newLine) != tc.expectCount {
				t.Errorf("Expected %d completions, got %d", tc.expectCount, len(newLine))
			}

			// Length should be reasonable for the context
			if length < 0 {
				t.Error("Completion length should not be negative")
			}
		})
	}
}

// Benchmark tests
func BenchmarkAutoCompleter_getCommandCandidates(b *testing.B) {
	app := createTestApp(&testing.T{})
	ac := NewAutoCompleter(app)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ac.getCommandCandidates("/co")
	}
}

func BenchmarkAutoCompleter_findCommonPrefix(b *testing.B) {
	app := createTestApp(&testing.T{})
	ac := NewAutoCompleter(app)

	candidates := []string{"hello", "help", "hero", "heart", "heavy"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ac.findCommonPrefix(candidates)
	}
}

func BenchmarkAutoCompleter_processCompletions(b *testing.B) {
	app := createTestApp(&testing.T{})
	ac := NewAutoCompleter(app)

	candidates := []string{"hello", "help", "hero"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ac.processCompletions(candidates, 2)
	}
}
