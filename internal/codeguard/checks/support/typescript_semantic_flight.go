package support

import "sync"

var (
	typeScriptSemanticCacheMu sync.Mutex
	typeScriptSemanticCache   = make(map[string]TypeScriptSemanticResults)
	typeScriptSemanticFlights = make(map[string]*typeScriptSemanticFlight)
)

type typeScriptSemanticFlight struct {
	done    chan struct{}
	results TypeScriptSemanticResults
	err     error
}

func typeScriptSemanticFlightFor(cacheKey string) (*typeScriptSemanticFlight, bool) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	if flight, ok := typeScriptSemanticFlights[cacheKey]; ok {
		return flight, false
	}
	flight := &typeScriptSemanticFlight{done: make(chan struct{})}
	typeScriptSemanticFlights[cacheKey] = flight
	return flight, true
}

func typeScriptSemanticFinishFlight(cacheKey string, flight *typeScriptSemanticFlight) {
	typeScriptSemanticCacheMu.Lock()
	delete(typeScriptSemanticFlights, cacheKey)
	close(flight.done)
	typeScriptSemanticCacheMu.Unlock()
}

func cachedTypeScriptSemanticResults(cacheKey string) (TypeScriptSemanticResults, bool) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	results, ok := typeScriptSemanticCache[cacheKey]
	return results, ok
}

func storeTypeScriptSemanticResults(cacheKey string, results TypeScriptSemanticResults) {
	typeScriptSemanticCacheMu.Lock()
	defer typeScriptSemanticCacheMu.Unlock()
	typeScriptSemanticCache[cacheKey] = results
}
