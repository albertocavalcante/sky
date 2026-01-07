package linter

import (
	"fmt"
	"sort"
	"strings"
)

// Registry manages a collection of lint rules with enable/disable controls.
type Registry struct {
	// rules maps rule names to their definitions
	rules map[string]*Rule

	// enabled tracks which rules are currently enabled
	enabled map[string]bool

	// configs holds per-rule configuration
	configs map[string]RuleConfig

	// categories maps category names to rule names
	categories map[string][]string
}

// NewRegistry creates a new empty registry.
func NewRegistry() *Registry {
	return &Registry{
		rules:      make(map[string]*Rule),
		enabled:    make(map[string]bool),
		configs:    make(map[string]RuleConfig),
		categories: make(map[string][]string),
	}
}

// Register adds rules to the registry and validates them.
// Returns an error if any rule has an invalid name or duplicates an existing rule.
func (r *Registry) Register(rules ...*Rule) error {
	for _, rule := range rules {
		if rule.Name == "" {
			return fmt.Errorf("rule has empty name")
		}

		// Check for duplicate names
		if _, exists := r.rules[rule.Name]; exists {
			return fmt.Errorf("duplicate rule name: %s", rule.Name)
		}

		// Validate rule name format (kebab-case)
		if !isValidRuleName(rule.Name) {
			return fmt.Errorf("invalid rule name %q: must be kebab-case (lowercase with hyphens)", rule.Name)
		}

		// Register the rule
		r.rules[rule.Name] = rule

		// Enable by default
		r.enabled[rule.Name] = true

		// Add to category index
		if rule.Category != "" {
			r.categories[rule.Category] = append(r.categories[rule.Category], rule.Name)
		}
	}

	return nil
}

// Enable enables the specified rules by name or category.
// Names can be exact rule names, category names, or "all".
// If a name matches both a rule and a category, the rule takes precedence.
func (r *Registry) Enable(names ...string) {
	for _, name := range names {
		if name == "all" {
			// Enable all rules
			for ruleName := range r.rules {
				r.enabled[ruleName] = true
			}
			continue
		}

		// Check if it's a specific rule
		if _, exists := r.rules[name]; exists {
			r.enabled[name] = true
			continue
		}

		// Check if it's a category
		if rules, exists := r.categories[name]; exists {
			for _, ruleName := range rules {
				r.enabled[ruleName] = true
			}
			continue
		}

		// Check if it's a glob pattern (e.g., "native-*")
		if strings.Contains(name, "*") {
			r.enablePattern(name)
		}
	}
}

// Disable disables the specified rules by name or category.
// Names can be exact rule names, category names, or "all".
// If a name matches both a rule and a category, the rule takes precedence.
func (r *Registry) Disable(names ...string) {
	for _, name := range names {
		if name == "all" {
			// Disable all rules
			for ruleName := range r.rules {
				r.enabled[ruleName] = false
			}
			continue
		}

		// Check if it's a specific rule
		if _, exists := r.rules[name]; exists {
			r.enabled[name] = false
			continue
		}

		// Check if it's a category
		if rules, exists := r.categories[name]; exists {
			for _, ruleName := range rules {
				r.enabled[ruleName] = false
			}
			continue
		}

		// Check if it's a glob pattern (e.g., "native-*")
		if strings.Contains(name, "*") {
			r.disablePattern(name)
		}
	}
}

// enablePattern enables all rules matching a glob pattern.
func (r *Registry) enablePattern(pattern string) {
	for ruleName := range r.rules {
		if matchGlob(pattern, ruleName) {
			r.enabled[ruleName] = true
		}
	}
}

// disablePattern disables all rules matching a glob pattern.
func (r *Registry) disablePattern(pattern string) {
	for ruleName := range r.rules {
		if matchGlob(pattern, ruleName) {
			r.enabled[ruleName] = false
		}
	}
}

// SetConfig sets the configuration for a specific rule.
func (r *Registry) SetConfig(ruleName string, config RuleConfig) error {
	if _, exists := r.rules[ruleName]; !exists {
		return fmt.Errorf("unknown rule: %s", ruleName)
	}
	r.configs[ruleName] = config
	return nil
}

// GetConfig returns the configuration for a specific rule.
// Returns an empty config if none is set.
func (r *Registry) GetConfig(ruleName string) RuleConfig {
	if config, exists := r.configs[ruleName]; exists {
		return config
	}
	return RuleConfig{}
}

// EnabledRules returns all currently enabled rules in dependency order.
// Rules with no dependencies come first, followed by rules that depend on them.
func (r *Registry) EnabledRules() []*Rule {
	var enabled []*Rule
	for name, rule := range r.rules {
		if r.enabled[name] {
			enabled = append(enabled, rule)
		}
	}

	// Sort by dependencies using topological sort
	return topologicalSort(enabled)
}

// AllRules returns all registered rules.
func (r *Registry) AllRules() []*Rule {
	rules := make([]*Rule, 0, len(r.rules))
	for _, rule := range r.rules {
		rules = append(rules, rule)
	}

	// Sort by name for consistent ordering
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Name < rules[j].Name
	})

	return rules
}

// Categories returns all known categories.
func (r *Registry) Categories() []string {
	cats := make([]string, 0, len(r.categories))
	for cat := range r.categories {
		cats = append(cats, cat)
	}
	sort.Strings(cats)
	return cats
}

// RulesByCategory returns all rules in a specific category.
func (r *Registry) RulesByCategory(category string) []*Rule {
	names, exists := r.categories[category]
	if !exists {
		return nil
	}

	rules := make([]*Rule, 0, len(names))
	for _, name := range names {
		if rule, exists := r.rules[name]; exists {
			rules = append(rules, rule)
		}
	}

	return rules
}

// Validate checks the registry for errors (cycles, missing dependencies).
func (r *Registry) Validate() error {
	// Check for cycles and missing dependencies
	for _, rule := range r.rules {
		if err := r.validateRule(rule, make(map[string]bool)); err != nil {
			return err
		}
	}
	return nil
}

// validateRule recursively checks a rule for dependency issues.
func (r *Registry) validateRule(rule *Rule, visited map[string]bool) error {
	// Check for cycles
	if visited[rule.Name] {
		return fmt.Errorf("dependency cycle detected involving rule: %s", rule.Name)
	}

	visited[rule.Name] = true
	defer delete(visited, rule.Name)

	// Check that all required rules exist
	for _, req := range rule.Requires {
		if _, exists := r.rules[req.Name]; !exists {
			return fmt.Errorf("rule %s requires unknown rule: %s", rule.Name, req.Name)
		}

		// Recursively validate dependencies
		if err := r.validateRule(req, visited); err != nil {
			return err
		}
	}

	return nil
}

// isValidRuleName checks if a rule name follows kebab-case convention.
// Allows lowercase letters, digits, hyphens, and underscores.
func isValidRuleName(name string) bool {
	if name == "" {
		return false
	}

	for i, ch := range name {
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= '0' && ch <= '9' && i > 0 {
			continue
		}
		if (ch == '-' || ch == '_') && i > 0 && i < len(name)-1 {
			continue
		}
		return false
	}

	return true
}

// matchGlob is a simple glob pattern matcher supporting only '*' wildcard.
func matchGlob(pattern, str string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == str
	}

	// Simple implementation: split on '*' and check prefix/suffix
	parts := strings.Split(pattern, "*")
	if len(parts) == 2 {
		prefix, suffix := parts[0], parts[1]
		return strings.HasPrefix(str, prefix) && strings.HasSuffix(str, suffix) &&
			len(str) >= len(prefix)+len(suffix)
	}

	// For more complex patterns, fall back to basic prefix/suffix matching
	if strings.HasPrefix(pattern, "*") {
		return strings.HasSuffix(str, strings.TrimPrefix(pattern, "*"))
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(str, strings.TrimSuffix(pattern, "*"))
	}

	return false
}

// topologicalSort sorts rules so dependencies come before dependents.
func topologicalSort(rules []*Rule) []*Rule {
	// Build adjacency map and in-degree count
	deps := make(map[string][]*Rule)  // rule -> rules that depend on it
	inDegree := make(map[string]int)  // rule -> number of dependencies
	ruleMap := make(map[string]*Rule) // name -> rule

	for _, rule := range rules {
		ruleMap[rule.Name] = rule
		if _, exists := inDegree[rule.Name]; !exists {
			inDegree[rule.Name] = 0
		}
		for _, req := range rule.Requires {
			deps[req.Name] = append(deps[req.Name], rule)
			inDegree[rule.Name]++
		}
	}

	// Kahn's algorithm for topological sort
	var queue []*Rule
	for _, rule := range rules {
		if inDegree[rule.Name] == 0 {
			queue = append(queue, rule)
		}
	}

	var sorted []*Rule
	for len(queue) > 0 {
		rule := queue[0]
		queue = queue[1:]
		sorted = append(sorted, rule)

		for _, dependent := range deps[rule.Name] {
			inDegree[dependent.Name]--
			if inDegree[dependent.Name] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// If we couldn't sort all rules, there's a cycle (shouldn't happen if Validate was called)
	if len(sorted) != len(rules) {
		// Fall back to original order
		return rules
	}

	return sorted
}
