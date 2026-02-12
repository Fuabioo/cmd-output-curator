package filter

// Registry holds filter strategies in priority order.
type Registry struct {
	strategies []Strategy
	fallback   Strategy
}

// NewRegistry creates a Registry with the given strategies and a passthrough fallback.
func NewRegistry(strategies ...Strategy) *Registry {
	return &Registry{
		strategies: strategies,
		fallback:   &PassthroughStrategy{},
	}
}

// Find returns the first strategy that can handle the command, or the fallback.
func (r *Registry) Find(command string, args []string) Strategy {
	for _, s := range r.strategies {
		if s.CanHandle(command, args) {
			return s
		}
	}
	return r.fallback
}

// DefaultRegistry returns a registry with all built-in strategies.
// Phase 2: git, go, and generic error filters.
func DefaultRegistry() *Registry {
	return NewRegistry(
		// Git strategies (most specific first)
		&GitStatusStrategy{},
		&GitDiffStrategy{},
		&GitLogStrategy{},
		// Go strategies
		&GoTestStrategy{},
		&GoBuildStrategy{},
		// Generic fallback (must be last among non-passthrough)
		&GenericErrorStrategy{},
	)
}
