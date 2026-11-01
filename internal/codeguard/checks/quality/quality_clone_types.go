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

// cloneWindow is one indexed threshold-token window. The indices are int32 to
// halve the index's footprint; both are bounded far below 2^31 by the clone
// corpus budget.
type cloneWindow struct {
	Hash       uint64
	DocIndex   int32
	TokenIndex int32
}

// cloneIndex holds the indexed windows sorted by hash, each hash's windows in
// (document, token) order.
type cloneIndex struct {
	windows []cloneWindow
}

// len reports how many windows the index holds.
func (index cloneIndex) len() int {
	return len(index.windows)
}
