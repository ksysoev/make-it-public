package api

type GenerateTokenRequest struct {
	KeyID string `json:"key_id"`
	TTL   uint   `json:"ttl"`
}

type GenerateTokenResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token"`
	KeyID   string `json:"key_id"`
	TTL     uint   `json:"ttl"`
}
