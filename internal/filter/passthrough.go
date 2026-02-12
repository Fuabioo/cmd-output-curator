package filter

// PassthroughStrategy returns output unchanged.
type PassthroughStrategy struct{}

func (p *PassthroughStrategy) Name() string {
	return "passthrough"
}

func (p *PassthroughStrategy) CanHandle(_ string, _ []string) bool {
	return true
}

func (p *PassthroughStrategy) Filter(raw []byte, _ string, _ []string, _ int) Result {
	return Result{
		Filtered:   string(raw),
		WasReduced: false,
	}
}
