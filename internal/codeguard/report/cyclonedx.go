package report

import (
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"

	"github.com/devr-tools/codeguard/internal/codeguard/core"
)

// writeCycloneDX serializes the normalized supply-chain artifacts into a
// CycloneDX 1.6 JSON BOM. It deliberately has no timestamp or generated UUID:
// an unchanged scan produces byte-for-byte identical output for cacheable CI.
func writeCycloneDX(w io.Writer, report core.Report) error {
	components := cycloneDXComponents(report.Artifacts)
	payload := cycloneDXBOM{
		Schema:      "http://cyclonedx.org/schema/bom-1.6.schema.json",
		BOMFormat:   "CycloneDX",
		SpecVersion: "1.6",
		Version:     1,
		Components:  components,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

type cycloneDXBOM struct {
	Schema      string               `json:"$schema"`
	BOMFormat   string               `json:"bomFormat"`
	SpecVersion string               `json:"specVersion"`
	Version     int                  `json:"version"`
	Components  []cycloneDXComponent `json:"components,omitempty"`
}

type cycloneDXComponent struct {
	Type       string              `json:"type"`
	BOMRef     string              `json:"bom-ref"`
	Name       string              `json:"name"`
	Version    string              `json:"version"`
	PURL       string              `json:"purl,omitempty"`
	Licenses   []cycloneDXLicense  `json:"licenses,omitempty"`
	Properties []cycloneDXProperty `json:"properties,omitempty"`
}

type cycloneDXLicense struct {
	License cycloneDXLicenseName `json:"license"`
}

type cycloneDXLicenseName struct {
	Name string `json:"name"`
}

type cycloneDXProperty struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func cycloneDXComponents(artifacts []core.Artifact) []cycloneDXComponent {
	components := make([]cycloneDXComponent, 0)
	for _, artifact := range artifacts {
		if artifact.SupplyChain == nil {
			continue
		}
		for _, manifest := range artifact.SupplyChain.Manifests {
			for _, dependency := range manifest.Dependencies {
				components = append(components, newCycloneDXComponent(manifest, dependency))
			}
		}
	}
	sort.Slice(components, func(i, j int) bool { return components[i].BOMRef < components[j].BOMRef })
	return components
}

func newCycloneDXComponent(manifest core.SupplyChainManifest, dependency core.SupplyChainDependency) cycloneDXComponent {
	version := strings.TrimSpace(dependency.Version)
	if version == "" {
		version = strings.TrimSpace(dependency.Requirement)
	}
	if version == "" {
		version = "unspecified"
	}
	ref := "codeguard:" + strings.TrimSpace(manifest.Ecosystem) + ":" + strings.TrimSpace(manifest.Path) + ":" + strings.TrimSpace(dependency.Name) + "@" + version
	component := cycloneDXComponent{
		Type:       "library",
		BOMRef:     ref,
		Name:       strings.TrimSpace(dependency.Name),
		Version:    version,
		PURL:       cycloneDXPURL(manifest.Ecosystem, dependency.Name, version),
		Properties: cycloneDXProperties(manifest, dependency),
	}
	if license := strings.TrimSpace(dependency.License); license != "" {
		component.Licenses = []cycloneDXLicense{{License: cycloneDXLicenseName{Name: license}}}
	}
	return component
}

func cycloneDXProperties(manifest core.SupplyChainManifest, dependency core.SupplyChainDependency) []cycloneDXProperty {
	properties := []cycloneDXProperty{
		{Name: "codeguard:ecosystem", Value: manifest.Ecosystem},
		{Name: "codeguard:manifest-path", Value: manifest.Path},
	}
	if scope := strings.TrimSpace(dependency.Scope); scope != "" {
		properties = append(properties, cycloneDXProperty{Name: "codeguard:scope", Value: scope})
	}
	if len(dependency.Groups) != 0 {
		groups := append([]string(nil), dependency.Groups...)
		sort.Strings(groups)
		properties = append(properties, cycloneDXProperty{Name: "codeguard:groups", Value: strings.Join(groups, ",")})
	}
	if requirement := strings.TrimSpace(dependency.Requirement); requirement != "" {
		properties = append(properties, cycloneDXProperty{Name: "codeguard:requirement", Value: requirement})
	}
	if dependency.Indirect {
		properties = append(properties, cycloneDXProperty{Name: "codeguard:indirect", Value: "true"})
	}
	return properties
}

func cycloneDXPURL(ecosystem string, name string, version string) string {
	packageType := map[string]string{
		"go":     "golang",
		"npm":    "npm",
		"node":   "npm",
		"python": "pypi",
		"cargo":  "cargo",
	}[strings.ToLower(strings.TrimSpace(ecosystem))]
	if packageType == "" || strings.TrimSpace(name) == "" || version == "unspecified" {
		return ""
	}
	return "pkg:" + packageType + "/" + cycloneDXPURLPath(name) + "@" + cycloneDXPURLValue(version)
}

// PURLs use URL escaping, while retaining path separators for package types
// such as Go and scoped npm packages. url.PathEscape deliberately leaves '@'
// alone, but PURL requires it to be encoded when it is part of a package name.
func cycloneDXPURLPath(value string) string {
	parts := strings.Split(strings.TrimSpace(value), "/")
	for i := range parts {
		parts[i] = cycloneDXPURLValue(parts[i])
	}
	return strings.Join(parts, "/")
}

func cycloneDXPURLValue(value string) string {
	escaped := url.PathEscape(strings.TrimSpace(value))
	return strings.ReplaceAll(escaped, "@", "%40")
}
