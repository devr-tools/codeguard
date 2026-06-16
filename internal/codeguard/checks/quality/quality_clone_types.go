package quality

type cloneToken struct {
	Value string
	Line  int
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
