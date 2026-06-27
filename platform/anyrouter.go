package platform

// AnyRouterAdapter inherits all the cookie retry and sign_in fallback logic
// from NewApiAdapter, matching the main app architecture.
type AnyRouterAdapter struct {
	NewApiAdapter
}

func init() {
	Register(&AnyRouterAdapter{NewApiAdapter: NewApiAdapter{BaseAdapter: BaseAdapter{Name: "anyrouter"}}})
}
