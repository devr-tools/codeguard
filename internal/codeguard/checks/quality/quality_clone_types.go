package quality

type cloneToken struct {
	// Value is the token's original source text (always ASCII: the token
	// pattern matches ASCII characters only). Comparisons fold ASCII case so
	// behavior matches the historical lowercased-token equality.
	Value string
	// Hash is the FNV-1a hash of the ASCII-lowercased token text, computed
	// once at tokenize time so window hashing never rehashes token bytes.
	Hash uint64
	Line int
}

type cloneDocument struct {
	Path   string
	Tokens []cloneToken
}

type cloneOccurrence struct {
	DocIndex   int
	TokenIndex int
}

type cloneCandidate struct {
	LeftDoc    int
	LeftStart  int
	RightDoc   int
	RightStart int
	Length     int
}

type cloneIndex map[uint64][]cloneOccurrence
