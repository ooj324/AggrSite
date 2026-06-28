package platform

import "fmt"

// OneHubAdapter extends one-api behavior.
type OneHubAdapter struct {
	OneApiAdapter
}

func init() {
	Register(&OneHubAdapter{OneApiAdapter: OneApiAdapter{BaseAdapter: BaseAdapter{Name: "one-hub"}}})
}

func (a *OneHubAdapter) PlatformName() string { return "one-hub" }

func (a *OneHubAdapter) GetModels(baseURL, accessToken string, platformUserID int64, opt *RequestOption) ([]string, error) {
	if models, err := a.OneApiAdapter.GetModels(baseURL, accessToken, platformUserID, opt); err == nil && len(models) > 0 {
		return models, nil
	}

	url := fmt.Sprintf("%s/api/available_model", baseURL)
	var res map[string]interface{}
	if err := a.FetchJSON(url, "GET", AuthHeaders(accessToken, platformUserID), nil, &res, opt); err != nil {
		return nil, err
	}

	payload := res
	if data, ok := res["data"].(map[string]interface{}); ok {
		payload = data
	}
	models := make([]string, 0, len(payload))
	for model := range payload {
		if model != "" {
			models = append(models, model)
		}
	}
	return models, nil
}
