package platform

import "strings"

var adapters []Adapter

func Register(a Adapter) {
	adapters = append(adapters, a)
}

func GetAdapter(platform string) Adapter {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	for _, a := range adapters {
		if strings.ToLower(a.PlatformName()) == normalized {
			return a
		}
	}
	return nil
}

func AllPlatformNames() []string {
	names := make([]string, len(adapters))
	for i, a := range adapters {
		names[i] = a.PlatformName()
	}
	return names
}
