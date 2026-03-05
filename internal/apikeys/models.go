package apikeys

type requestNewApiKey struct {
	Name string `json:"name"`
}

type newApiKey struct {
	ApiKeyName string `json:"api_key_name"`
	ApiKeyHash string `json:"api_key_hash"`
}

type apiKeyAction struct {
	ApiKeyID string `json:"api_key_id"`
}
