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
	systemPrefixes := []string{"/etc", "/usr", "/var", "/opt", "/System"}
	for _, prefix := range systemPrefixes {
		if strings.HasPrefix(absPath, prefix) {
			return SecurityFileTypeSystem
		}
	}
	
	// Check for user home directory
	homeDir, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(absPath, homeDir) {
		// Check if it's in a project subdirectory within home
		// Look for common project indicators
		projectIndicators := []string{
			"/.git/", "/node_modules/", "/.vscode/", 
			"/src/", "/pkg/", "/cmd/", "/internal/",
		}
		
		for _, indicator := range projectIndicators {
			if strings.Contains(absPath, indicator) {
				return SecurityFileTypeProject
			}
		}
		
		// Check for typical project files
		dir := filepath.Dir(absPath)
		projectFiles := []string{
			"go.mod", "package.json", "Cargo.toml", 
			"requirements.txt", "Makefile", "duck.yaml",
		}
		
		for _, projectFile := range projectFiles {
			if _, err := os.Stat(filepath.Join(dir, projectFile)); err == nil {
				return SecurityFileTypeProject
			}
		}
		
		// If in home directory but no project indicators, it's a user config
		return SecurityFileTypeUser
	}
	
	// Default to project type for any other paths
	return SecurityFileTypeProject
}

// FilePermissionResult holds the result of file permission validation
type FilePermissionResult struct {
	Path                string
	Type                SecurityFileType
	Valid               bool
	Issues              []string
	Owner               string
	Group               string
	Permissions         os.FileMode
	ParentDirSecure     bool
	ParentDirIssues     []string
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
	
	// Get file info
	fileInfo, err := os.Stat(configFile.Path)
	if err != nil {
		result.Issues = append(result.Issues, fmt.Sprintf("failed to stat file: %v", err))
		return result, nil
	}
	
	result.Permissions = fileInfo.Mode()
	
	// Get owner and group information (Unix-like systems only)
	if runtime.GOOS != "windows" {
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			result.Owner = fmt.Sprintf("uid:%d", stat.Uid)
			result.Group = fmt.Sprintf("gid:%d", stat.Gid)
			
			// Validate ownership based on file type and policy
			if policy.EnforceOwnership {
				if err := validateOwnership(configFile.Type, stat, result); err != nil {
					result.Issues = append(result.Issues, err.Error())
				}
			}
		}
	}
	
	// Validate file permissions
	if err := validateFileMode(fileInfo.Mode(), policy, result); err != nil {
		result.Issues = append(result.Issues, err.Error())
	}
	
	// Validate parent directory security if required
	if policy.RequireSecureDirectories {
		parentResult, err := validateParentDirectories(configFile.Path, policy)
		if err != nil {
			result.ParentDirIssues = append(result.ParentDirIssues, fmt.Sprintf("failed to validate parent directories: %v", err))
		} else {
			result.ParentDirSecure = parentResult.Secure
			result.ParentDirIssues = parentResult.Issues
		}
	} else {
		result.ParentDirSecure = true // Not required, so considered secure
	}
	
	// Overall validation status
	result.Valid = len(result.Issues) == 0 && (result.ParentDirSecure || !policy.RequireSecureDirectories)
	
	return result, nil
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
		currentUid := uint32(os.Getuid())
		if stat.Uid != currentUid {
			return fmt.Errorf("user config file should be owned by current user (uid %d), but is owned by uid %d", currentUid, stat.Uid)
		}
		
	case SecurityFileTypeProject:
		// Project files can be owned by current user or project team
		// More flexible ownership rules for collaborative development
		currentUid := uint32(os.Getuid())
		if stat.Uid != currentUid {
			// Allow if file is in a shared project directory
			// This is a more permissive check for development workflows
			result.Issues = append(result.Issues, fmt.Sprintf("project config file owned by uid %d instead of current user uid %d (warning)", stat.Uid, currentUid))
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
		if err := validateSingleDirectory(dir, policy, result); err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("directory %s: %v", dir, err))
			result.Secure = false
		}
		
		dir = filepath.Dir(dir)
	}
	
	return result, nil
}

// validateSingleDirectory validates a single directory's permissions
func validateSingleDirectory(dirPath string, policy *FilePermissionPolicy, result *ParentDirectoryResult) error {
	dirInfo, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("failed to stat directory: %w", err)
	}
	
	if !dirInfo.IsDir() {
		return fmt.Errorf("path is not a directory")
	}
	
	perm := dirInfo.Mode() & os.ModePerm
	
	// Directory should not be writable by others
	if perm&0002 != 0 {
		return fmt.Errorf("directory writable by others (mode: %o)", perm)
	}
	
	// Check group write permissions based on policy
	if !policy.AllowGroupWrite && perm&0020 != 0 {
		return fmt.Errorf("directory writable by group but group write not allowed (mode: %o)", perm)
	}
	
	// On Unix-like systems, check ownership
	if runtime.GOOS != "windows" {
		if stat, ok := dirInfo.Sys().(*syscall.Stat_t); ok {
			currentUid := uint32(os.Getuid())
			
			// For system paths, require root ownership
			if isSystemPath(dirPath) && stat.Uid != 0 {
				return fmt.Errorf("system directory should be owned by root, but owned by uid %d", stat.Uid)
			}
			
			// For user paths, require current user ownership
			if isUserPath(dirPath) && stat.Uid != currentUid {
				return fmt.Errorf("user directory should be owned by current user (uid %d), but owned by uid %d", currentUid, stat.Uid)
			}
		}
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
	var targetMode os.FileMode
	switch result.Type {
	case SecurityFileTypeSystem:
		if policy.EnforceReadOnly {
			targetMode = 0644 // rw-r--r--
		} else {
			targetMode = 0644 // rw-r--r--
		}
		
	case SecurityFileTypeUser:
		if policy.EnforceReadOnly {
			if policy.AllowGroupWrite {
				targetMode = 0664 // rw-rw-r--
			} else {
				targetMode = 0644 // rw-r--r--
			}
		} else {
			if policy.AllowGroupWrite {
				targetMode = 0664 // rw-rw-r--
			} else {
				targetMode = 0644 // rw-r--r--
			}
		}
		
	case SecurityFileTypeProject:
		if policy.AllowGroupWrite {
			targetMode = 0664 // rw-rw-r--
		} else {
			targetMode = 0644 // rw-r--r--
		}
	}
	
	// Apply the permission fix
	if err := os.Chmod(result.Path, targetMode); err != nil {
		return fmt.Errorf("failed to change permissions on %s: %w", result.Path, err)
	}
	
	fmt.Printf("Fixed permissions for %s (mode: %o)\n", result.Path, targetMode)
	return nil
}