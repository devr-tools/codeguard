package report

import "github.com/devr-tools/codeguard/internal/codeguard/core"

type sarifTaxonomy struct {
	Name             string         `json:"name"`
	Version          string         `json:"version"`
	GUID             string         `json:"guid"`
	ShortDescription sarifTextBlock `json:"shortDescription"`
	Taxa             []sarifTaxon   `json:"taxa"`
	InformationURI   string         `json:"informationUri,omitempty"`
	IsComprehensive  bool           `json:"isComprehensive"`
}

type sarifTaxon struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	ShortDescription sarifTextBlock `json:"shortDescription"`
}

type sarifTextBlock struct {
	Text string `json:"text"`
}

func owaspTaxonomy() sarifTaxonomy {
	taxa := make([]sarifTaxon, 0, len(core.OWASPTop10))
	for _, category := range core.OWASPTop10 {
		taxa = append(taxa, sarifTaxon{
			ID:               category.Code(),
			Name:             category.Name(),
			ShortDescription: sarifTextBlock{Text: string(category)},
		})
	}
	return sarifTaxonomy{
		Name:             owaspTaxonomyName,
		Version:          "2021",
		GUID:             owaspTaxonomyGUID,
		ShortDescription: sarifTextBlock{Text: "OWASP Top 10 (2021)"},
		InformationURI:   "https://owasp.org/Top10/",
		IsComprehensive:  true,
		Taxa:             taxa,
	}
}
