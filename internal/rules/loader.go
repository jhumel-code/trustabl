package rules

import (
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"github.com/trustabl/karenctl/internal/models"
)

// Load reads all .yaml files from fsys, unmarshals and validates each, and
// returns all policy files. All errors are collected — not fail-fast — so a
// contributor sees every problem in one run.
func Load(fsys fs.FS) ([]PolicyFile, error) {
	entries, err := fs.Glob(fsys, "*.yaml")
	if err != nil {
		return nil, fmt.Errorf("glob: %w", err)
	}

	var (
		policies []PolicyFile
		errs     []error
		seenIDs  = map[string]string{} // rule ID → file that defined it
	)

	for _, name := range entries {
		f, err := fsys.Open(name)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: open: %w", name, err))
			continue
		}
		defer f.Close()

		var pf PolicyFile
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		decErr := dec.Decode(&pf)

		if decErr != nil {
			errs = append(errs, fmt.Errorf("%s: decode: %w", name, decErr))
			continue
		}

		// Validate policy-level required fields.
		policyErrCount := len(errs)
		if pf.Policy.ID == "" {
			errs = append(errs, fmt.Errorf("%s: policy.id is required", name))
		}
		if pf.Policy.Category == "" {
			errs = append(errs, fmt.Errorf("%s: policy.category is required", name))
		}
		if pf.Policy.Category != "" {
			switch pf.Policy.Category {
			case models.CategoryClaudeSDK, models.CategoryOpenShell:
				// valid
			default:
				errs = append(errs, fmt.Errorf("%s: unknown category %q (allowed: claude_sdk, openshell)", name, pf.Policy.Category))
			}
		}
		if len(errs) > policyErrCount {
			continue
		}

		for i, rule := range pf.Rules {
			tag := fmt.Sprintf("%s rule[%d]", name, i)
			if rule.ID != "" {
				tag = fmt.Sprintf("%s rule %s", name, rule.ID)
			}
			if rule.ID == "" {
				errs = append(errs, fmt.Errorf("%s: id is required", tag))
			}
			if rule.Title == "" {
				errs = append(errs, fmt.Errorf("%s: title is required", tag))
			}
			if rule.Severity == "" {
				errs = append(errs, fmt.Errorf("%s: severity is required", tag))
			}
			if rule.Severity != "" {
				switch rule.Severity {
				case models.SeverityInfo, models.SeverityLow, models.SeverityMedium,
					models.SeverityHigh, models.SeverityCritical:
					// valid
				default:
					errs = append(errs, fmt.Errorf("%s: unknown severity %q (allowed: info, low, medium, high, critical)", tag, rule.Severity))
				}
			}
			if rule.Confidence <= 0 {
				errs = append(errs, fmt.Errorf("%s: confidence is required (must be > 0)", tag))
			}
			if len(rule.AppliesTo) == 0 {
				errs = append(errs, fmt.Errorf("%s: applies_to is required", tag))
			}
			if rule.Explanation == "" {
				errs = append(errs, fmt.Errorf("%s: explanation is required", tag))
			}
			if rule.Fix == "" {
				errs = append(errs, fmt.Errorf("%s: fix is required", tag))
			}
			if rule.ID != "" {
				if prev, seen := seenIDs[rule.ID]; seen {
					errs = append(errs, fmt.Errorf("duplicate rule ID %q in %s (previously defined in %s)", rule.ID, name, prev))
				} else {
					seenIDs[rule.ID] = name
				}
			}
			// Populate category from policy metadata — not in YAML.
			pf.Rules[i].Category = models.DetectorCategory(pf.Policy.Category)
		}
		policies = append(policies, pf)
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return policies, nil
}
