package platform

import "strings"

var adapters []Adapter

func Register(a Adapter) {
	adapters = append(adapters, a)
}

func normalizePlatformName(platform string) string {
	return strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.ToLower(strings.TrimSpace(platform)))
}

func GetAdapter(platform string) Adapter {
	normalized := normalizePlatformName(platform)
	for _, a := range adapters {
		if normalizePlatformName(a.PlatformName()) == normalized {
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
