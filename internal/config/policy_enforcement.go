package config

import (
	"fmt"
	"strings"
)

// PolicyViolation represents a security policy violation
type PolicyViolation struct {
	Type       string // "checksum", "commit_tracking", "auto_update", "template_validation"
	Severity   string // "error", "warning", "info"
	Message    string
	TargetName string
	Suggestion string // How to fix the violation
}

// PolicyEnforcementResult contains the results of policy enforcement checks
type PolicyEnforcementResult struct {
	Allowed    bool
	Violations []PolicyViolation
	Warnings   []PolicyViolation
}

// NewPolicyEnforcementResult creates a new result with allowed=true by default
func NewPolicyEnforcementResult() *PolicyEnforcementResult {
	return &PolicyEnforcementResult{
		Allowed:    true,
		Violations: make([]PolicyViolation, 0),
		Warnings:   make([]PolicyViolation, 0),
	}
}

// AddViolation adds a policy violation and sets Allowed=false
func (r *PolicyEnforcementResult) AddViolation(violationType, message, targetName, suggestion string) {
	r.Allowed = false
	r.Violations = append(r.Violations, PolicyViolation{
		Type:       violationType,
		Severity:   "error",
		Message:    message,
		TargetName: targetName,
		Suggestion: suggestion,
	})
}

// AddWarning adds a policy warning (doesn't affect Allowed status)
func (r *PolicyEnforcementResult) AddWarning(violationType, message, targetName, suggestion string) {
	r.Warnings = append(r.Warnings, PolicyViolation{
		Type:       violationType,
		Severity:   "warning",
		Message:    message,
		TargetName: targetName,
		Suggestion: suggestion,
	})
}

// EnforceSecurityPolicies validates a target against security policies
// Returns a result indicating if the target is allowed and any violations found
func EnforceSecurityPolicies(targetName string, target *Target, securityCfg *SecurityConfig) *PolicyEnforcementResult {
	result := NewPolicyEnforcementResult()

	// Skip enforcement if no security config or no enforcement policy
	if securityCfg == nil || securityCfg.Enforcement == nil {
		return result
	}

	enforcement := securityCfg.Enforcement

	// 1. Checksum Validation Enforcement
	if enforcement.ForceChecksumValidation {
		if err := enforceChecksumValidation(targetName, target, result); err != nil {
			result.AddViolation("checksum_validation", err.Error(), targetName,
				"Add a checksum field to the template configuration")
		}
	}

	// 2. Commit Tracking Enforcement
	if enforcement.ForceCommitTracking {
		if err := enforceCommitTracking(targetName, target, result); err != nil {
			result.AddViolation("commit_tracking", err.Error(), targetName,
				"Set trackCommitHash: true in the template configuration")
		}
	}

	// 3. Auto-Update Controls
	if enforcement.DisableAutoUpdate {
		if err := enforceAutoUpdateDisabled(targetName, target, result); err != nil {
			result.AddWarning("auto_update_override", err.Error(), targetName,
				"This setting is being overridden by security policy")
		}
	}

	// 4. Template Validation
	if err := validateTemplateConfiguration(targetName, target, result); err != nil {
		result.AddViolation("template_validation", err.Error(), targetName,
			"Fix the template configuration as indicated")
	}

	// 5. Repository Access Validation (reuse existing logic)
	if err := ValidateRepoAccess(target.Template.Repo, securityCfg); err != nil {
		result.AddViolation("repository_access", err.Error(), targetName,
			"Use an allowed repository or update security configuration")
	}

	return result
}

// enforceChecksumValidation checks if checksum is required and present
func enforceChecksumValidation(targetName string, target *Target, result *PolicyEnforcementResult) error {
	if strings.TrimSpace(target.Template.Checksum) == "" {
		return fmt.Errorf("security policy requires checksum validation but no checksum provided for target %q", targetName)
	}
	return nil
}

// enforceCommitTracking checks if commit tracking is required and enabled
func enforceCommitTracking(targetName string, target *Target, result *PolicyEnforcementResult) error {
	if !target.Template.TrackCommitHash {
		return fmt.Errorf("security policy requires commit tracking but trackCommitHash is disabled for target %q", targetName)
	}
	return nil
}

// enforceAutoUpdateDisabled checks and warns about auto-update overrides
func enforceAutoUpdateDisabled(targetName string, target *Target, result *PolicyEnforcementResult) error {
	if target.Template.AutoUpdateOnChange {
		return fmt.Errorf("security policy disables auto-updates but autoUpdateOnChange is enabled for target %q", targetName)
	}
	return nil
}

// validateTemplateConfiguration performs additional template validation
func validateTemplateConfiguration(targetName string, target *Target, result *PolicyEnforcementResult) error {
	// Validate repository URL format
	if strings.TrimSpace(target.Template.Repo) == "" {
		return fmt.Errorf("template repository is required for target %q", targetName)
	}

	// Validate git reference
	if strings.TrimSpace(target.Template.Ref) == "" {
		return fmt.Errorf("template git reference is required for target %q", targetName)
	}

	// Validate template path
	if strings.TrimSpace(target.Template.Path) == "" {
		return fmt.Errorf("template path is required for target %q", targetName)
	}

	// Check for suspicious git references (security concern)
	suspiciousRefs := []string{"HEAD", "master", "main"}
	for _, suspiciousRef := range suspiciousRefs {
		if target.Template.Ref == suspiciousRef {
			result.AddWarning("template_validation",
				fmt.Sprintf("target %q uses potentially unstable git reference %q - consider using a specific tag or commit", targetName, suspiciousRef),
				targetName,
				"Use a specific tag or commit hash instead of branch names")
		}
	}

	return nil
}

// ApplyPolicyOverrides modifies target configuration based on security policies
// This function applies mandatory overrides that cannot be bypassed
func ApplyPolicyOverrides(target *Target, securityCfg *SecurityConfig) *Target {
	// Create a copy to avoid modifying the original
	modifiedTarget := *target

	// Make a deep copy of the template to avoid modifying the original
	var modifiedTemplate *Template
	if target.Template != nil {
		templateCopy := *target.Template
		modifiedTemplate = &templateCopy
	}

	if securityCfg != nil && securityCfg.Enforcement != nil {
		enforcement := securityCfg.Enforcement

		// Force disable auto-update if policy requires it
		if enforcement.DisableAutoUpdate && modifiedTemplate != nil {
			modifiedTemplate.AutoUpdateOnChange = false
		}

		// Force enable commit tracking if policy requires it
		if enforcement.ForceCommitTracking && modifiedTemplate != nil {
			modifiedTemplate.TrackCommitHash = true
		}
	}

	modifiedTarget.Template = modifiedTemplate
	return &modifiedTarget
}

// ValidateStrictPolicyMode checks if security configuration is required and present
func ValidateStrictPolicyMode(securityCfg *SecurityConfig) error {
	// If no security config at all, but we're being called, that might be an issue
	if securityCfg == nil {
		return fmt.Errorf("strict policy mode requires a security configuration but none was found")
	}

	// If enforcement is configured and strict policy mode is enabled
	if securityCfg.Enforcement != nil && securityCfg.Enforcement.StrictPolicyMode {
		// Validate that the security config has meaningful policies
		if !hasAnyPolicyEnforcement(securityCfg) {
			return fmt.Errorf("strict policy mode is enabled but no security policies are configured")
		}

		// Require signed configuration in strict mode
		if !securityCfg.IsSigned && securityCfg.Enforcement.StrictPolicyMode {
			return fmt.Errorf("strict policy mode requires a digitally signed security configuration")
		}
	}

	return nil
}

// hasAnyPolicyEnforcement checks if any meaningful policy enforcement is configured
func hasAnyPolicyEnforcement(securityCfg *SecurityConfig) bool {
	if securityCfg == nil || securityCfg.Enforcement == nil {
		return false
	}

	enforcement := securityCfg.Enforcement

	// Check if any enforcement policies are enabled
	return enforcement.ForceChecksumValidation ||
		enforcement.ForceCommitTracking ||
		enforcement.DisableAutoUpdate ||
		len(securityCfg.AllowedHosts) > 0 ||
		len(securityCfg.DeniedHosts) > 0 ||
		securityCfg.StrictMode
}

// FormatPolicyViolations formats policy violations for user display
func FormatPolicyViolations(result *PolicyEnforcementResult) string {
	if result == nil || (len(result.Violations) == 0 && len(result.Warnings) == 0) {
		return ""
	}

	var output strings.Builder

	// Format violations (errors)
	if len(result.Violations) > 0 {
		output.WriteString("Security Policy Violations:\n")
		for i, violation := range result.Violations {
			output.WriteString(fmt.Sprintf("  %d. %s\n", i+1, violation.Message))
			if violation.Suggestion != "" {
				output.WriteString(fmt.Sprintf("     Suggestion: %s\n", violation.Suggestion))
			}
		}
	}

	// Format warnings
	if len(result.Warnings) > 0 {
		if len(result.Violations) > 0 {
			output.WriteString("\n")
		}
		output.WriteString("Security Policy Warnings:\n")
		for i, warning := range result.Warnings {
			output.WriteString(fmt.Sprintf("  %d. %s\n", i+1, warning.Message))
			if warning.Suggestion != "" {
				output.WriteString(fmt.Sprintf("     Suggestion: %s\n", warning.Suggestion))
			}
		}
	}

	return output.String()
}

// GetPolicyEnforcementSummary provides a summary of active policy enforcement
func GetPolicyEnforcementSummary(securityCfg *SecurityConfig) string {
	if securityCfg == nil {
		return "No security policies enforced"
	}

	var policies []string

	// Check enforcement policies if they exist
	if securityCfg.Enforcement != nil {
		enforcement := securityCfg.Enforcement

		if enforcement.ForceChecksumValidation {
			policies = append(policies, "Checksum validation required")
		}

		if enforcement.ForceCommitTracking {
			policies = append(policies, "Commit tracking required")
		}

		if enforcement.DisableAutoUpdate {
			policies = append(policies, "Auto-updates disabled")
		}

		if enforcement.StrictPolicyMode {
			policies = append(policies, "Strict policy mode enabled")
		}

		if enforcement.EnforceFilePermissions {
			policies = append(policies, "File permissions enforced")
		}
	}

	// Check host restrictions (these are policies even without enforcement struct)
	if len(securityCfg.AllowedHosts) > 0 {
		policies = append(policies, fmt.Sprintf("Repository access limited to %d hosts", len(securityCfg.AllowedHosts)))
	}

	if len(securityCfg.DeniedHosts) > 0 {
		policies = append(policies, fmt.Sprintf("%d hosts explicitly denied", len(securityCfg.DeniedHosts)))
	}

	if len(policies) == 0 {
		return "No security policies enforced"
	}

	return fmt.Sprintf("Active policies: %s", strings.Join(policies, ", "))
}
