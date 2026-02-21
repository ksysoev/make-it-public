package api

type GenerateTokenRequest struct {
	KeyID string `json:"key_id"`
	Type  string `json:"type"`
	TTL   int    `json:"ttl"`
}

type GenerateTokenResponse struct {
	Token string `json:"token"`
	KeyID string `json:"key_id"`
	Type  string `json:"type"`
	TTL   int    `json:"ttl"`
}
