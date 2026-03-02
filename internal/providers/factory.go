package providers

import (
	"fmt"
	"sync"
)

// FactoryFunc constructs a Provider from config fields.
type FactoryFunc func(name, url, token string, priority int) (Provider, error)

var (
	factoryMu sync.RWMutex
	factories  = map[string]FactoryFunc{}
)

// RegisterFactory registers a constructor for the given provider type string.
// Panics if the type is already registered (like database/sql.Register).
func RegisterFactory(typeName string, fn FactoryFunc) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	if _, exists := factories[typeName]; exists {
		panic(fmt.Sprintf("providers: factory %q already registered", typeName))
	}
	factories[typeName] = fn
}

// Build constructs a Provider using the registered factory for the given type.
func Build(typeName, name, url, token string, priority int) (Provider, error) {
	factoryMu.RLock()
	fn, ok := factories[typeName]
	factoryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider type: %q", typeName)
	}
	return fn(name, url, token, priority)
}
