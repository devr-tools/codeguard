package config

import (
	"fmt"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

func validateDesignArchitectureRules(rules core.DesignRulesConfig) error {
	layers, err := validateDesignLayers(rules.Layers)
	if err != nil {
		return err
	}
	err = validateLayerReferences(rules.Layers, layers)
	if err != nil {
		return err
	}
	domains, err := validateDesignDomains(rules.Domains)
	if err != nil {
		return err
	}
	err = validateDomainReferences(rules.Domains, domains)
	if err != nil {
		return err
	}
	err = validateDesignCapabilities(rules.Capabilities)
	if err != nil {
		return err
	}
	err = validateDesignPublicSurfaces(rules.PublicSurfaces)
	if err != nil {
		return err
	}
	return validateDesignBoundaryToggles(rules)
}

func validateDesignLayers(ruleLayers []core.DesignLayerConfig) (map[string]struct{}, error) {
	layers := make(map[string]struct{}, len(ruleLayers))
	for idx, layer := range ruleLayers {
		name := strings.TrimSpace(layer.Name)
		if name == "" {
			return nil, fmt.Errorf("design_rules.layers[%d].name is required", idx)
		}
		key := strings.ToLower(name)
		if _, exists := layers[key]; exists {
			return nil, fmt.Errorf("design_rules.layers contains duplicate name %q", name)
		}
		if !hasNonBlankDesignValue(layer.Paths) {
			return nil, fmt.Errorf("design_rules.layers[%d].paths must not be empty", idx)
		}
		layers[key] = struct{}{}
	}
	return layers, nil
}

func validateLayerReferences(ruleLayers []core.DesignLayerConfig, layers map[string]struct{}) error {
	for idx, layer := range ruleLayers {
		for _, dependency := range append(append([]string(nil), layer.MayDependOn...), layer.DenyDependOn...) {
			if _, ok := layers[strings.ToLower(strings.TrimSpace(dependency))]; !ok {
				return fmt.Errorf("design_rules.layers[%d] references unknown layer %q", idx, dependency)
			}
		}
	}
	return nil
}

func validateDesignDomains(ruleDomains []core.DesignDomainConfig) (map[string]struct{}, error) {
	domains := make(map[string]struct{}, len(ruleDomains))
	for idx, domain := range ruleDomains {
		name := strings.TrimSpace(domain.Name)
		if name == "" {
			return nil, fmt.Errorf("design_rules.domains[%d].name is required", idx)
		}
		key := strings.ToLower(name)
		if _, exists := domains[key]; exists {
			return nil, fmt.Errorf("design_rules.domains contains duplicate name %q", name)
		}
		if !hasNonBlankDesignValue(domain.Paths) {
			return nil, fmt.Errorf("design_rules.domains[%d].paths must not be empty", idx)
		}
		domains[key] = struct{}{}
	}
	return domains, nil
}

func validateDomainReferences(ruleDomains []core.DesignDomainConfig, domains map[string]struct{}) error {
	for idx, domain := range ruleDomains {
		for _, dependency := range domain.MayDependOn {
			if _, ok := domains[strings.ToLower(strings.TrimSpace(dependency))]; !ok {
				return fmt.Errorf("design_rules.domains[%d] references unknown domain %q", idx, dependency)
			}
		}
	}
	return nil
}

func validateDesignCapabilities(capabilities []core.DesignCapabilityConfig) error {
	seenCapabilities := map[string]struct{}{}
	for idx, capability := range capabilities {
		name := strings.TrimSpace(capability.Name)
		if name == "" {
			return fmt.Errorf("design_rules.capabilities[%d].name is required", idx)
		}
		key := strings.ToLower(name)
		if _, exists := seenCapabilities[key]; exists {
			return fmt.Errorf("design_rules.capabilities contains duplicate name %q", name)
		}
		if !hasNonBlankDesignValue(capability.Imports) || !hasNonBlankDesignValue(capability.AllowedPaths) {
			return fmt.Errorf("design_rules.capabilities[%d].imports and allowed_paths must not be empty", idx)
		}
		seenCapabilities[key] = struct{}{}
	}
	return nil
}

func validateDesignPublicSurfaces(surfaces []core.DesignPublicSurfaceConfig) error {
	seenSurfaces := map[string]struct{}{}
	for idx, surface := range surfaces {
		name := strings.TrimSpace(surface.Name)
		if name == "" {
			return fmt.Errorf("design_rules.public_surfaces[%d].name is required", idx)
		}
		key := strings.ToLower(name)
		if _, exists := seenSurfaces[key]; exists {
			return fmt.Errorf("design_rules.public_surfaces contains duplicate name %q", name)
		}
		if !hasNonBlankDesignValue(surface.Paths) || !hasNonBlankDesignValue(surface.Entrypoints) {
			return fmt.Errorf("design_rules.public_surfaces[%d].paths and entrypoints must not be empty", idx)
		}
		seenSurfaces[key] = struct{}{}
	}
	return nil
}

func validateDesignBoundaryToggles(rules core.DesignRulesConfig) error {
	if policy := rules.ProductionTest; policy != nil && designConfigEnabled(policy.Enabled) {
		if !hasNonBlankDesignValue(policy.ProductionPaths) || !hasNonBlankDesignValue(policy.TestPaths) {
			return fmt.Errorf("design_rules.production_test.production_paths and test_paths must not be empty")
		}
	}
	if policy := rules.Stability; policy != nil {
		if policy.MinimumFanIn < 0 {
			return fmt.Errorf("design_rules.stability.minimum_fan_in must not be negative")
		}
		if policy.MaxInstabilityDelta < 0 || policy.MaxInstabilityDelta > 1 {
			return fmt.Errorf("design_rules.stability.max_instability_delta must be between 0 and 1")
		}
	}
	return nil
}

func designConfigEnabled(value *bool) bool {
	return value == nil || *value
}

func hasNonBlankDesignValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
