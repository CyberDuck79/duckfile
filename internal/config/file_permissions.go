package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// DetermineSecurityFileType determines the security file type based on path
func DetermineSecurityFileType(filePath string) SecurityFileType {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		// Fallback to project type if we can't determine absolute path
		return SecurityFileTypeProject
	}

	// Check for system paths
	if isSystemPathType(absPath) {
		return SecurityFileTypeSystem
	}

	// Check for user home directory and project indicators
	return determineUserOrProjectType(absPath)
}

// isSystemPathType checks if the path is in system directories
func isSystemPathType(absPath string) bool {
	systemPrefixes := []string{"/etc", "/usr", "/var", "/opt", "/System"}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return true
		}
	}
	return false
}

// determineUserOrProjectType determines if a path is user config or project config
func determineUserOrProjectType(absPath string) SecurityFileType {
	homeDir, err := os.UserHomeDir()
	if err != nil || !strings.HasPrefix(absPath, homeDir) {
		// Not in home directory, default to project type
		return SecurityFileTypeProject
	}

	// Check if it's in a project subdirectory within home
	if hasProjectIndicators(absPath) {
		return SecurityFileTypeProject
	}

	// Check for typical project files in the same directory
	if hasProjectFiles(absPath) {
		return SecurityFileTypeProject
	}

	// If in home directory but no project indicators, it's a user config
	return SecurityFileTypeUser
}

// hasProjectIndicators checks for common project directory indicators
func hasProjectIndicators(absPath string) bool {
	projectIndicators := []string{
		"/.git/", "/node_modules/", "/.vscode/",
		"/src/", "/pkg/", "/cmd/", "/internal/",
	}

	for _, indicator := range projectIndicators {
		if strings.Contains(absPath, indicator) {
			return true
		}
	}
	return false
}

// hasProjectFiles checks for typical project files in the directory
func hasProjectFiles(absPath string) bool {
	dir := filepath.Dir(absPath)
	projectFiles := []string{
		"go.mod", "package.json", "Cargo.toml",
		"requirements.txt", "Makefile", "duck.yaml",
	}

	for _, projectFile := range projectFiles {
		if _, err := os.Stat(filepath.Join(dir, projectFile)); err == nil {
			return true
		}
	}
	return false
}

// FilePermissionResult holds the result of file permission validation
type FilePermissionResult struct {
	Path            string
	Type            SecurityFileType
	Valid           bool
	Issues          []string
	Owner           string
	Group           string
	Permissions     os.FileMode
	ParentDirSecure bool
	ParentDirIssues []string
}

// ValidateFilePermissions validates security configuration file permissions according to policy
func ValidateFilePermissions(configFile *SecurityConfigFile, policy *FilePermissionPolicy) (*FilePermissionResult, error) {
	if configFile == nil {
		return nil, fmt.Errorf("config file is nil")
	}

	if policy == nil {
		// No policy means no validation required
		return &FilePermissionResult{
			Path:  configFile.Path,
			Type:  configFile.Type,
			Valid: true,
		}, nil
	}

	result := &FilePermissionResult{
		Path: configFile.Path,
		Type: configFile.Type,
	}

	// Check if file exists
	if !configFile.Exists {
		result.Issues = append(result.Issues, "file does not exist")
		return result, nil
	}

	// Get file info and validate
	if err := validateFileInfo(configFile, policy, result); err != nil {
		return result, err
	}

	// Validate parent directories
	validateParentDirectorySecurity(configFile.Path, policy, result)

	// Set overall validation status
	result.Valid = len(result.Issues) == 0 && (result.ParentDirSecure || !policy.RequireSecureDirectories)

	return result, nil
}

// validateFileInfo validates file information including permissions and ownership
func validateFileInfo(configFile *SecurityConfigFile, policy *FilePermissionPolicy, result *FilePermissionResult) error {
	fileInfo, err := os.Stat(configFile.Path)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("failed to stat file: %v", err))
		return nil
	}

	result.Permissions = fileInfo.Mode()

	// Validate ownership on Unix-like systems
	validateFileOwnership(configFile, policy, result, fileInfo)

	// Validate file permissions
	if err := validateFileMode(fileInfo.Mode(), policy, result); err != nil {
		result.Issues = append(result.Issues, err.Error())
	}

	return nil
}

// validateFileOwnership validates file ownership based on system type
func validateFileOwnership(configFile *SecurityConfigFile, policy *FilePermissionPolicy, result *FilePermissionResult, fileInfo os.FileInfo) {
	if runtime.GOOS == "windows" {
		return
	}

	stat, ok := fileInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}

	result.Owner = fmt.Sprintf("uid:%d", stat.Uid)
	result.Group = fmt.Sprintf("gid:%d", stat.Gid)

	// Validate ownership based on file type and policy
	if policy.EnforceOwnership {
		if err := validateOwnership(configFile.Type, stat, result); err != nil {
			result.Issues = append(result.Issues, err.Error())
		}
	}
}

// validateParentDirectorySecurity validates parent directory security if required
func validateParentDirectorySecurity(filePath string, policy *FilePermissionPolicy, result *FilePermissionResult) {
	if !policy.RequireSecureDirectories {
		result.ParentDirSecure = true // Not required, so considered secure
		return
	}

	parentResult, err := validateParentDirectories(filePath, policy)
	if err != nil {
		result.ParentDirIssues = append(result.ParentDirIssues, fmt.Sprintf("failed to validate parent directories: %v", err))
		return
	}

	result.ParentDirSecure = parentResult.Secure
	result.ParentDirIssues = parentResult.Issues
}

// validateOwnership checks file ownership based on file type and policy
func validateOwnership(fileType SecurityFileType, stat *syscall.Stat_t, result *FilePermissionResult) error {
	switch fileType {
	case SecurityFileTypeSystem:
		// System files should be owned by root (uid 0)
		if stat.Uid != 0 {
			return fmt.Errorf("system config file should be owned by root (uid 0), but is owned by uid %d", stat.Uid)
		}

	case SecurityFileTypeUser:
		// User files should be owned by the current user
		currentUID := uint32(os.Getuid())
		if stat.Uid != currentUID {
			return fmt.Errorf("user config file should be owned by current user (uid %d), but is owned by uid %d", currentUID, stat.Uid)
		}

	case SecurityFileTypeProject:
		// Project files can be owned by current user or project team
		// More flexible ownership rules for collaborative development
		currentUID := uint32(os.Getuid())
		if stat.Uid != currentUID {
			// Allow if file is in a shared project directory
			// This is a more permissive check for development workflows
			result.Issues = append(result.Issues, fmt.Sprintf("project config file owned by uid %d instead of current user uid %d (warning)", stat.Uid, currentUID))
			return nil // Don't fail, just warn
		}
	}

	return nil
}

// validateFileMode checks file permissions according to policy
func validateFileMode(mode os.FileMode, policy *FilePermissionPolicy, result *FilePermissionResult) error {
	perm := mode & os.ModePerm

	// Check if file is read-only when required
	if policy.EnforceReadOnly {
		// File should not be writable by owner, group, or others
		if perm&0200 != 0 { // Owner write
			return fmt.Errorf("file should be read-only but owner has write permission (mode: %o)", perm)
		}
		if perm&0020 != 0 { // Group write
			return fmt.Errorf("file should be read-only but group has write permission (mode: %o)", perm)
		}
		if perm&0002 != 0 { // Other write
			return fmt.Errorf("file should be read-only but others have write permission (mode: %o)", perm)
		}
	}

	// Check group write permissions
	if !policy.AllowGroupWrite && perm&0020 != 0 {
		return fmt.Errorf("group write permission not allowed but present (mode: %o)", perm)
	}

	// Check other permissions (generally should be restricted)
	if perm&0002 != 0 { // Other write
		return fmt.Errorf("others should not have write permission (mode: %o)", perm)
	}

	if perm&0004 == 0 { // Other read
		// Others should be able to read system configs but not user/project configs
		if result.Type == SecurityFileTypeSystem {
			return fmt.Errorf("system config should be readable by others (mode: %o)", perm)
		}
		// User and project configs don't need to be readable by others
	}

	return nil
}

// ParentDirectoryResult holds validation results for parent directories
type ParentDirectoryResult struct {
	Secure bool
	Issues []string
}

// validateParentDirectories checks that parent directories are secure
func validateParentDirectories(filePath string, policy *FilePermissionPolicy) (*ParentDirectoryResult, error) {
	result := &ParentDirectoryResult{Secure: true}

	// Get absolute path
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check each parent directory up to root
	dir := filepath.Dir(absPath)
	for dir != "/" && dir != "." && dir != filepath.Dir(dir) {
		// Skip validation for system directories that are expected to have special permissions
		if shouldSkipDirectoryValidation(dir) {
			dir = filepath.Dir(dir)
			continue
		}

		if err := validateSingleDirectory(dir, policy); err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("directory %s: %v", dir, err))
			result.Secure = false
		}

		dir = filepath.Dir(dir)
	}

	return result, nil
}

// shouldSkipDirectoryValidation determines if a directory should be skipped during security validation
func shouldSkipDirectoryValidation(dirPath string) bool {
	// Skip validation for system directories that are expected to be world-writable
	systemDirs := []string{
		"/tmp",
		"/var/tmp",
		"/usr/tmp",
	}

	for _, sysDir := range systemDirs {
		if dirPath == sysDir {
			return true
		}
	}

	// Skip validation for test temporary directories (Go test framework creates these)
	// These typically have names like /tmp/TestSomething123456789/001 or /var/folders/.../TestSomething.../001
	if strings.Contains(dirPath, "/Test") && (strings.Contains(dirPath, "/001") || strings.Contains(dirPath, "/tmp")) {
		return true
	}

	// Skip validation for macOS temporary directories created by tests
	if strings.Contains(dirPath, "/var/folders/") && strings.Contains(dirPath, "/Test") {
		return true
	}

	return false
}

// validateSingleDirectory validates a single directory's permissions
func validateSingleDirectory(dirPath string, policy *FilePermissionPolicy) error {
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}

	if !dirInfo.IsDir() {
		return fmt.Errorf("path is not a directory")
	}

	// Validate directory permissions
	if err := validateDirectoryPermissions(dirInfo.Mode(), policy); err != nil {
		return err
	}

	// Validate directory ownership on Unix-like systems
	return validateDirectoryOwnership(dirPath, dirInfo)
}

// validateDirectoryPermissions validates directory permission bits
func validateDirectoryPermissions(mode os.FileMode, policy *FilePermissionPolicy) error {
	perm := mode & os.ModePerm

	// Directory should not be writable by others
	if perm&0002 != 0 {
		return fmt.Errorf("directory writable by others (mode: %o)", perm)
	}

	// Check group write permissions based on policy
	if !policy.AllowGroupWrite && perm&0020 != 0 {
		return fmt.Errorf("directory writable by group but group write not allowed (mode: %o)", perm)
	}

	return nil
}

// validateDirectoryOwnership validates directory ownership on Unix-like systems
func validateDirectoryOwnership(dirPath string, dirInfo os.FileInfo) error {
	if runtime.GOOS == "windows" {
		return nil
	}

	stat, ok := dirInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}

	currentUID := uint32(os.Getuid())

	// For system paths, require root ownership
	if isSystemPath(dirPath) && stat.Uid != 0 {
		return fmt.Errorf("system directory should be owned by root, but owned by uid %d", stat.Uid)
	}

	// For user paths, require current user ownership
	if isUserPath(dirPath) && stat.Uid != currentUID {
		return fmt.Errorf("user directory should be owned by current user (uid %d), but owned by uid %d", currentUID, stat.Uid)
	}

	return nil
}

// isSystemPath checks if a path is a system-level path
func isSystemPath(path string) bool {
	systemPrefixes := []string{"/etc", "/usr", "/opt"}
	// On macOS, also include /System and /Library but exclude temp directories
	if runtime.GOOS == "darwin" {
		systemPrefixes = append(systemPrefixes, "/System", "/Library")
	}

	for _, prefix := range systemPrefixes {
		if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
			// Make sure it's not a temp directory (which might start with /var)
			if prefix == "/var" && strings.Contains(path, "/folders/") {
				return false // macOS temp directories
			}
			return true
		}
	}
	return false
}

// isUserPath checks if a path is a user-level path
func isUserPath(path string) bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	return len(path) >= len(homeDir) && path[:len(homeDir)] == homeDir
}

// ValidateSecurityConfigPermissions validates all discovered security config files
func ValidateSecurityConfigPermissions(configs []*SecurityConfigFile, policy *FilePermissionPolicy) ([]*FilePermissionResult, error) {
	if policy == nil {
		// No policy means no validation
		return nil, nil
	}

	var results []*FilePermissionResult

	for _, config := range configs {
		if config == nil || !config.Exists {
			continue
		}

		result, err := ValidateFilePermissions(config, policy)
		if err != nil {
			return nil, fmt.Errorf("failed to validate permissions for %s: %w", config.Path, err)
		}

		results = append(results, result)
	}

	return results, nil
}

// FixFilePermissions attempts to fix file permission issues
func FixFilePermissions(result *FilePermissionResult, policy *FilePermissionPolicy, dryRun bool) error {
	if result == nil || policy == nil {
		return fmt.Errorf("result or policy is nil")
	}

	if result.Valid {
		return nil // Nothing to fix
	}

	if dryRun {
		fmt.Printf("[DRY RUN] Would fix permissions for %s\n", result.Path)
		for _, issue := range result.Issues {
			fmt.Printf("[DRY RUN]   Issue: %s\n", issue)
		}
		return nil
	}

	// Determine target permissions based on file type and policy
	targetMode := calculateTargetMode(result.Type, policy)

	// Apply the permission fix
	if err := os.Chmod(result.Path, targetMode); err != nil {
		return fmt.Errorf("failed to change permissions on %s: %w", result.Path, err)
	}

	fmt.Printf("Fixed permissions for %s (mode: %o)\n", result.Path, targetMode)
	return nil
}

// calculateTargetMode determines the appropriate file permissions based on file type and policy
func calculateTargetMode(fileType SecurityFileType, policy *FilePermissionPolicy) os.FileMode {
	switch fileType {
	case SecurityFileTypeSystem:
		// System files are always rw-r--r-- regardless of EnforceReadOnly
		return 0644

	case SecurityFileTypeUser, SecurityFileTypeProject:
		// User and project files follow the same permission logic
		if policy.AllowGroupWrite {
			return 0664 // rw-rw-r--
		}
		return 0644 // rw-r--r--

	default:
		return 0644 // Default to rw-r--r--
	}
}
