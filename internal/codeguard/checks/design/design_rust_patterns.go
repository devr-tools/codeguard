package design

import "regexp"

var (
	rustTraitPattern       = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:unsafe\s+)?trait\s+([A-Za-z_]\w*)\b`)
	rustImplPattern        = regexp.MustCompile(`^\s*impl\b`)
	rustMethodPattern      = regexp.MustCompile(`^\s*(?:pub(?:\([^)]*\))?\s+)?(?:default\s+)?(?:async\s+)?(?:const\s+)?(?:unsafe\s+)?fn\s+([A-Za-z_]\w*)\b`)
	rustTraitMemberPattern = regexp.MustCompile(`^\s*(?:(?:default\s+)?(?:async\s+)?(?:const\s+)?(?:unsafe\s+)?fn\s+[A-Za-z_]\w*\b|type\s+[A-Za-z_]\w*\b|const\s+[A-Za-z_]\w*\b)`)
)
