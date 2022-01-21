package cache

type SSOClientCredentials struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
	ExpiresAt    string `json:"expiresAt"`
}

type SSOToken struct {
	StartUrl    string `json:"startUrl"`
	Region      string `json:"region"`
	AccessToken string `json:"accessToken"`
	ExpiresAt   string `json:"expiresAt"`
}
