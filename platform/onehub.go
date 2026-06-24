package platform

// OneHubAdapter extends one-api behavior.
type OneHubAdapter struct {
	OneApiAdapter
}

func init() {
	Register(&OneHubAdapter{OneApiAdapter: OneApiAdapter{BaseAdapter: BaseAdapter{Name: "one-hub"}}})
}

func (a *OneHubAdapter) PlatformName() string { return "one-hub" }
