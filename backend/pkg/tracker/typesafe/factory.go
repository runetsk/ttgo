package typesafe

// ClientFactory builds TypeSafe clients that share one process-wide Limiter, so every caller
// (failure-analysis jobs, sync analyze, Explain, the settings connection test) draws from the
// same request budget. A nil *ClientFactory is valid and builds unlimited clients.
type ClientFactory struct {
	limiter Limiter
}

// NewClientFactory returns a factory whose clients wait on l (nil = unlimited).
func NewClientFactory(l Limiter) *ClientFactory {
	return &ClientFactory{limiter: l}
}

// New builds a client with opts; the factory's limiter is used unless opts.Limiter is set.
func (f *ClientFactory) New(apiKey string, opts Options) *HTTPClient {
	if f != nil && opts.Limiter == nil {
		opts.Limiter = f.limiter
	}
	return NewHTTPClient(apiKey, opts)
}
