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
		return ac.getCommands(), 0
	}

	// Handle different completion contexts
	var candidates []string
	var completionLength int

	switch {
	case strings.HasPrefix(lineStr, "/connect ") && len(words) > 1:
		candidates = ac.getConnectionCandidates(words, lineStr)
		completionLength = ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "/describe ") && len(words) > 1:
		candidates = ac.getTableCandidates(words, lineStr)
		completionLength = ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "@"):
		candidates = ac.getFileCandidates(lineStr)
		completionLength = ac.getCompletionLength(lineStr)
	case strings.HasPrefix(lineStr, "/exec ") && strings.Contains(lineStr, " > "):
		candidates = ac.getCSVCandidates(words, lineStr)
		completionLength = ac.getCompletionLength(lineStr)
	case strings.Contains(lineStr, " > ") && !strings.HasPrefix(lineStr, "/"):
		candidates = ac.getCSVCandidates(words, lineStr)
		completionLength = ac.getCompletionLength(lineStr)
	case len(words) == 1 && strings.HasPrefix(words[0], "/"):
		// Command completion for partial commands like /co -> /connect
		candidates = ac.getCommandCandidates(words[0])
		completionLength = len(words[0])
	default:
		return nil, 0
	}

	return ac.processCompletions(candidates, completionLength), completionLength
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

// processCompletions handles intelligent completion with common prefix
func (ac *AutoCompleter) processCompletions(candidates []string, typedLength int) [][]rune {
	if len(candidates) == 0 {
		return nil
	}

	if len(candidates) == 1 {
		// Single match - return the completion part
		return [][]rune{[]rune(candidates[0])}
	}

	// Multiple matches - find common prefix
	commonPrefix := ac.findCommonPrefix(candidates)

	if commonPrefix != "" {
		// Return the common prefix as a single completion
		return [][]rune{[]rune(commonPrefix)}
	}

	// No common prefix - return all candidates for user to choose
	result := make([][]rune, len(candidates))
	for i, candidate := range candidates {
		result[i] = []rune(candidate)
	}
	return result
}

// findCommonPrefix finds the longest common prefix among candidates
func (ac *AutoCompleter) findCommonPrefix(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}

	if len(candidates) == 1 {
		return candidates[0]
	}

	// Find the shortest candidate to limit our search
	minLen := len(candidates[0])
	for _, candidate := range candidates[1:] {
		if len(candidate) < minLen {
			minLen = len(candidate)
		}
	}

	// Find common prefix
	var commonPrefix strings.Builder
	for i := 0; i < minLen; i++ {
		char := candidates[0][i]
		allMatch := true

		for _, candidate := range candidates[1:] {
			if candidate[i] != char {
				allMatch = false
				break
			}
		}

		if !allMatch {
			break
		}

		commonPrefix.WriteByte(char)
	}

	return commonPrefix.String()
}


// New candidate-getting functions that return full matches for intelligent processing
func (ac *AutoCompleter) getCommandCandidates(partial string) []string {
	commands := []string{
		"/help", "/quit", "/exit", "/connect", "/list-connections",
		"/tables", "/describe", "/status", "/exec",
	}

	var candidates []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, partial) {
			// Return the completion part (what should be appended)
			completion := cmd[len(partial):]
			candidates = append(candidates, completion)
		}
	}

	return candidates
}

func (ac *AutoCompleter) getConnectionCandidates(words []string, line string) []string {
	if len(words) < 2 {
		return nil
	}

	connections, err := ac.app.configMgr.ListConnections()
	if err != nil {
		return nil
	}

	var candidates []string
	currentWord := ""
	if len(words) > 1 {
		currentWord = words[len(words)-1]
	}

	for _, conn := range connections {
		if strings.HasPrefix(conn.Name, currentWord) {
			// Return the completion part
			completion := conn.Name[len(currentWord):]
			candidates = append(candidates, completion)
		}
	}

	return candidates
}

func (ac *AutoCompleter) getTableCandidates(words []string, line string) []string {
	if len(words) < 2 || ac.app.connection == nil {
		return nil
	}

	tables, err := ac.app.connection.ListTables()
	if err != nil {
		return nil
	}

	var candidates []string
	currentWord := ""
	if len(words) > 1 {
		currentWord = words[len(words)-1]
	}

	for _, table := range tables {
		if strings.HasPrefix(table, currentWord) {
			// Return the completion part
			completion := table[len(currentWord):]
			candidates = append(candidates, completion)
		}
	}

	return candidates
}

func (ac *AutoCompleter) getFileCandidates(line string) []string {
	// Remove the @ prefix
	path := strings.TrimPrefix(line, "@")

	var candidates []string

	// If path is empty, show all .sql files and directories from current directory
	if path == "" {
		ac.addRecursiveFileCandidates(&candidates, ".", "", "")
		return candidates
	}

	// If path ends with /, it's a directory - show its contents
	if strings.HasSuffix(path, "/") {
		dir := strings.TrimSuffix(path, "/")
		if dir == "" {
			dir = "."
		}
		ac.addFileCandidates(&candidates, dir, "", path)
		return candidates
	}

	// Get the directory and filename parts
	dir := filepath.Dir(path)
	if dir == "." {
		dir = ""
	}

	baseName := filepath.Base(path)

	// If path contains directory, search in that specific directory
	if dir != "" {
		ac.addFileCandidates(&candidates, dir, baseName, dir+"/")
	} else {
		// Search recursively from current directory
		ac.addRecursiveFileCandidates(&candidates, ".", baseName, "")
	}

	return candidates
}

func (ac *AutoCompleter) getCSVCandidates(words []string, line string) []string {
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

	var candidates []string

	// Search in current directory
	ac.addCSVCandidates(&candidates, ".", baseName, "")

	// If filename contains directory, search in that directory
	if dir != "" && dir != "." {
		ac.addCSVCandidates(&candidates, dir, baseName, dir+"/")
	}

	return candidates
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



// New candidate-based helper functions for intelligent completion
func (ac *AutoCompleter) addFileCandidates(candidates *[]string, dir, baseName, prefix string) {
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
			// Return only the part that should be appended
			completion := name[len(baseName):]

			// Only suggest .sql files and directories
			if entry.IsDir() {
				*candidates = append(*candidates, completion+"/")
			} else if strings.HasSuffix(name, ".sql") {
				*candidates = append(*candidates, completion)
			}
		}
	}
}

func (ac *AutoCompleter) addRecursiveFileCandidates(candidates *[]string, dir, baseName, prefix string) {
	// First add files and directories in current directory
	ac.addFileCandidates(candidates, dir, baseName, prefix)

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

		// For recursive search, we need to show files with full paths
		// but only return the completion part
		subDir := filepath.Join(dir, name)

		// Find all matching files in subdirectory
		ac.addRecursiveFileCandidateMatches(candidates, subDir, baseName, name+"/")
	}
}

func (ac *AutoCompleter) addRecursiveFileCandidateMatches(candidates *[]string, dir, baseName, pathPrefix string) {
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
			// For recursive matches, return the full path completion
			fullCompletion := pathPrefix + name
			if baseName != "" {
				// Remove the baseName part since it's already typed
				fullCompletion = pathPrefix + name[len(baseName):]
			}

			if entry.IsDir() {
				*candidates = append(*candidates, fullCompletion+"/")
			} else if strings.HasSuffix(name, ".sql") {
				*candidates = append(*candidates, fullCompletion)
			}
		}

		// Continue searching subdirectories
		if entry.IsDir() && !ac.shouldSkipDirectory(name) {
			subDir := filepath.Join(dir, name)
			ac.addRecursiveFileCandidateMatches(candidates, subDir, baseName, pathPrefix+name+"/")
		}
	}
}

func (ac *AutoCompleter) addCSVCandidates(candidates *[]string, dir, baseName, prefix string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, baseName) {
			// Return only the part that should be appended
			completion := name[len(baseName):]

			if entry.IsDir() {
				*candidates = append(*candidates, completion+"/")
			} else {
				*candidates = append(*candidates, completion)
			}
		}
	}

	// Also suggest .csv extension if not already present
	if baseName != "" && !strings.HasSuffix(baseName, ".csv") {
		*candidates = append(*candidates, ".csv")
	}
}

func (ac *AutoCompleter) getCompletionLength(line string) int {
	words := strings.Fields(line)
	if len(words) == 0 {
		return 0
	}

	// For file completions starting with @
	if strings.HasPrefix(line, "@") {
		return len(strings.TrimPrefix(line, "@"))
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
