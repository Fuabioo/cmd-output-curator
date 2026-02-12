package filter

// Result holds the outcome of filtering.
type Result struct {
	Filtered   string
	WasReduced bool
}

// Strategy is the interface all command filters implement.
type Strategy interface {
	Name() string
	CanHandle(command string, args []string) bool
	Filter(raw []byte, command string, args []string, exitCode int) Result
}
