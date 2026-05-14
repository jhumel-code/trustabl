package rules

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

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

		var pf PolicyFile
		dec := yaml.NewDecoder(f)
		dec.KnownFields(true)
		decErr := dec.Decode(&pf)
		f.Close()

		if decErr != nil {
			errs = append(errs, fmt.Errorf("%s: decode: %w", name, decErr))
			continue
		}

		// Validate policy-level required fields.
		if pf.Policy.ID == "" {
			errs = append(errs, fmt.Errorf("%s: policy.id is required", name))
		}
		if pf.Policy.Category == "" {
			errs = append(errs, fmt.Errorf("%s: policy.category is required", name))
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
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return nil, errors.New(strings.Join(msgs, "\n"))
	}
	return policies, nil
}
