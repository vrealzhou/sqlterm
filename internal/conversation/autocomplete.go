package conversation

import (
	"os"
	"path/filepath"
	"strings"
)

type AutoCompleter struct {
	app *App
}

func NewAutoCompleter(app *App) *AutoCompleter {
	return &AutoCompleter{app: app}
}

func (ac *AutoCompleter) Do(line []rune, pos int) (newLine [][]rune, length int) {
	lineStr := string(line)
	words := strings.Fields(lineStr)
	
	if len(words) == 0 {
		return ac.getCommands(), len(lineStr)
	}

	// Handle different completion contexts
	switch {
	case strings.HasPrefix(lineStr, "/connect "):
		return ac.getConnectionCompletions(words, lineStr), ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "/describe "):
		return ac.getTableCompletions(words, lineStr), ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "@"):
		return ac.getFileCompletions(lineStr), ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "/exec ") && strings.Contains(lineStr, " > "):
		return ac.getCSVCompletions(words, lineStr), ac.getCompletionLength(lineStr)
	case strings.Contains(lineStr, " > ") && !strings.HasPrefix(lineStr, "/"):
		return ac.getCSVCompletions(words, lineStr), ac.getCompletionLength(lineStr)
	case len(words) == 1 && strings.HasPrefix(words[0], "/"):
		return ac.getCommands(), len(words[0])
	}

	return nil, 0
}

func (ac *AutoCompleter) getCommands() [][]rune {
	commands := []string{
		"/help", "/quit", "/exit", "/connect", "/list-connections", 
		"/tables", "/describe", "/status", "/exec",
	}
	
	result := make([][]rune, len(commands))
	for i, cmd := range commands {
		result[i] = []rune(cmd)
	}
	return result
}

func (ac *AutoCompleter) getConnectionCompletions(words []string, line string) [][]rune {
	if len(words) < 2 {
		return nil
	}

	connections, err := ac.app.configMgr.ListConnections()
	if err != nil {
		return nil
	}

	var completions [][]rune
	currentWord := ""
	if len(words) > 1 {
		currentWord = words[len(words)-1]
	}

	for _, conn := range connections {
		if strings.HasPrefix(conn.Name, currentWord) {
			completions = append(completions, []rune(conn.Name))
		}
	}

	return completions
}

func (ac *AutoCompleter) getTableCompletions(words []string, line string) [][]rune {
	if len(words) < 2 || ac.app.connection == nil {
		return nil
	}

	tables, err := ac.app.connection.ListTables()
	if err != nil {
		return nil
	}

	var completions [][]rune
	currentWord := ""
	if len(words) > 1 {
		currentWord = words[len(words)-1]
	}

	for _, table := range tables {
		if strings.HasPrefix(table, currentWord) {
			completions = append(completions, []rune(table))
		}
	}

	return completions
}

func (ac *AutoCompleter) getFileCompletions(line string) [][]rune {
	// Remove the @ prefix
	path := strings.TrimPrefix(line, "@")
	
	var completions [][]rune
	
	// If path is empty, show all .sql files and directories from current directory
	if path == "" {
		ac.addRecursiveFileCompletions(&completions, ".", "", "")
		return completions
	}
	
	// If path ends with /, it's a directory - show its contents
	if strings.HasSuffix(path, "/") {
		dir := strings.TrimSuffix(path, "/")
		if dir == "" {
			dir = "."
		}
		ac.addFileCompletions(&completions, dir, "", path)
		return completions
	}
	
	// Get the directory and filename parts
	dir := filepath.Dir(path)
	if dir == "." {
		dir = ""
	}
	
	baseName := filepath.Base(path)
	
	// If path contains directory, search in that specific directory
	if dir != "" {
		ac.addFileCompletions(&completions, dir, baseName, dir+"/")
	} else {
		// Search recursively from current directory
		ac.addRecursiveFileCompletions(&completions, ".", baseName, "")
	}

	return completions
}

func (ac *AutoCompleter) addFileCompletions(completions *[][]rune, dir, baseName, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		
		// Skip hidden files and directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		
		if strings.HasPrefix(name, baseName) {
			// Only suggest .sql files and directories
			if entry.IsDir() {
				*completions = append(*completions, []rune(prefix+name+"/"))
			} else if strings.HasSuffix(name, ".sql") {
				*completions = append(*completions, []rune(prefix+name))
			}
		}
	}
}

func (ac *AutoCompleter) addRecursiveFileCompletions(completions *[][]rune, dir, baseName, prefix string) {
	// First add files and directories in current directory
	ac.addFileCompletions(completions, dir, baseName, prefix)
	
	// Then recursively search subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		
		name := entry.Name()
		
		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}
		
		// Skip common non-relevant directories
		if ac.shouldSkipDirectory(name) {
			continue
		}
		
		// Recursively search subdirectory
		subDir := filepath.Join(dir, name)
		subPrefix := prefix + name + "/"
		ac.addRecursiveFileCompletions(completions, subDir, baseName, subPrefix)
	}
}

func (ac *AutoCompleter) shouldSkipDirectory(name string) bool {
	// Skip common directories that are unlikely to contain SQL files
	skipDirs := []string{
		"node_modules", ".git", ".svn", ".hg", "vendor", "target", 
		"build", "dist", "bin", "obj", ".vscode", ".idea",
		"__pycache__", ".pytest_cache", ".coverage", "coverage",
		"logs", "tmp", "temp", ".DS_Store", "Thumbs.db",
	}
	
	for _, skipDir := range skipDirs {
		if name == skipDir {
			return true
		}
	}
	
	return false
}

func (ac *AutoCompleter) getCSVCompletions(words []string, line string) [][]rune {
	// Find the part after ">"
	parts := strings.Split(line, " > ")
	if len(parts) < 2 {
		return nil
	}

	filename := strings.TrimSpace(parts[1])
	
	// Get directory and basename
	dir := filepath.Dir(filename)
	if dir == "." {
		dir = ""
	}
	
	baseName := filepath.Base(filename)
	if filename == "" || strings.HasSuffix(filename, "/") {
		baseName = ""
	}

	var completions [][]rune
	
	// Search in current directory
	ac.addCSVCompletions(&completions, ".", baseName, "")
	
	// If filename contains directory, search in that directory
	if dir != "" && dir != "." {
		ac.addCSVCompletions(&completions, dir, baseName, dir+"/")
	}

	return completions
}

func (ac *AutoCompleter) addCSVCompletions(completions *[][]rune, dir, baseName, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, baseName) {
			if entry.IsDir() {
				*completions = append(*completions, []rune(prefix+name+"/"))
			} else {
				*completions = append(*completions, []rune(prefix+name))
			}
		}
	}
	
	// Also suggest .csv extension if not already present
	if baseName != "" && !strings.HasSuffix(baseName, ".csv") {
		*completions = append(*completions, []rune(prefix+baseName+".csv"))
	}
}

func (ac *AutoCompleter) getCompletionLength(line string) int {
	words := strings.Fields(line)
	if len(words) == 0 {
		return 0
	}
	
	// For file completions starting with @
	if strings.HasPrefix(line, "@") {
		return len(line) - 1 // Exclude the @
	}
	
	// For CSV completions, return the length of the filename part
	if strings.Contains(line, " > ") {
		parts := strings.Split(line, " > ")
		if len(parts) >= 2 {
			return len(strings.TrimSpace(parts[1]))
		}
	}
	
	// For other completions, return the length of the last word
	return len(words[len(words)-1])
}