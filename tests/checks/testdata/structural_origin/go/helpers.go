package structuralorigin

import (
	"fmt"
	"sort"
	"strings"
)

type BadgeDefinitionRule struct {
	MetricSource string
	MetricName   string
	Operator     string
}

type BadgeInput struct {
	Key        string
	Name       string
	Rule       BadgeDefinitionRule
	SubjectIDs []string
}

type StringPatch struct {
	Set   bool
	Value *string
}

type BrandCampaignPatchInput struct {
	Name     StringPatch
	Currency StringPatch
}

type UpdatePlaceInput struct {
	Name    StringPatch
	Website StringPatch
}

type OvertureDivision struct {
	ID          string
	Name        string
	CountryCode string
	Hierarchy   []string
}

type StaticOvertureDivisionResolver struct {
	divisions []OvertureDivision
}

type duckDBOvertureDivisionRow struct {
	Name        string
	CountryCode string
	RegionCode  string
}

func normalizeBadgeRule(rule BadgeDefinitionRule) BadgeDefinitionRule {
	rule.MetricSource = strings.ToLower(strings.TrimSpace(rule.MetricSource))
	rule.MetricName = strings.TrimSpace(rule.MetricName)
	rule.Operator = strings.ToLower(strings.TrimSpace(rule.Operator))
	return rule
}

func normalizeUpdateBadgeInput(input BadgeInput) BadgeInput {
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	input.Name = strings.TrimSpace(input.Name)
	input.Rule = normalizeBadgeRule(input.Rule)
	return input
}

func normalizePreviewBadgeInput(input BadgeInput) BadgeInput {
	input.Key = strings.ToLower(strings.TrimSpace(input.Key))
	input.SubjectIDs = append([]string(nil), input.SubjectIDs...)
	return input
}

func normalizeRecomputeBadgeInput(input BadgeInput) BadgeInput {
	input.SubjectIDs = append([]string(nil), input.SubjectIDs...)
	return input
}

func normalizeBrandCampaignPatchInput(input BrandCampaignPatchInput) BrandCampaignPatchInput {
	if input.Name.Set && input.Name.Value != nil {
		value := strings.TrimSpace(*input.Name.Value)
		input.Name.Value = &value
	}
	if input.Currency.Set && input.Currency.Value != nil {
		currency := strings.ToUpper(strings.TrimSpace(*input.Currency.Value))
		input.Currency.Value = &currency
	}
	return input
}

func normalizePlaceUpdateFields(input UpdatePlaceInput) UpdatePlaceInput {
	normalized := input
	if normalized.Name.Set && normalized.Name.Value != nil {
		value := strings.TrimSpace(*normalized.Name.Value)
		normalized.Name.Value = &value
	}
	if normalized.Website.Set && normalized.Website.Value != nil {
		value := strings.TrimSpace(*normalized.Website.Value)
		normalized.Website.Value = &value
	}
	return normalized
}

func OvertureDatasetPath(release string, theme string, typ string) string {
	release = strings.TrimSpace(release)
	theme = strings.TrimSpace(theme)
	typ = strings.TrimSpace(typ)
	return fmt.Sprintf("s3://release/%s/theme=%s/type=%s/*", release, theme, typ)
}

func NewStaticOvertureDivisionResolver(divisions []OvertureDivision) *StaticOvertureDivisionResolver {
	copied := append([]OvertureDivision(nil), divisions...)
	for i := range copied {
		copied[i].ID = strings.TrimSpace(copied[i].ID)
		copied[i].Name = strings.TrimSpace(copied[i].Name)
	}
	sort.SliceStable(copied, func(i int, j int) bool {
		return copied[i].Name < copied[j].Name
	})
	return &StaticOvertureDivisionResolver{divisions: copied}
}

func duckDBDivisionHierarchy(row duckDBOvertureDivisionRow) []string {
	values := []string{row.CountryCode, row.RegionCode, row.Name}
	hierarchy := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		hierarchy = append(hierarchy, value)
	}
	return hierarchy
}

type Counter struct {
	value int
}

func (counter *Counter) CurrentReceiver() int {
	counter.value++
	return counter.value
}

func CurrentArgument(counter *Counter) int {
	counter.value++
	return counter.value
}

var sharedCounter Counter

func CurrentGlobal() int {
	sharedCounter.value++
	return sharedCounter.value
}

var escapedCounter *Counter

func CurrentEscaped() *Counter {
	counter := &Counter{}
	escapedCounter = counter
	counter.value++
	return counter
}

func CurrentUnresolvedGo() int {
	mystery.Update()
	return 1
}
