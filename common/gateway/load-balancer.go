package gateway

type LoadBalancer struct {
	ID                string   `json:"_id"`
	Name              string   `json:"name"`
	ApplicationIDs    []string `json:"applicationIDs"`
	StickinessOptions struct {
		Stickiness     bool     `json:"stickiness"`
		Duration       int      `json:"duration"`
		UseRPCID       bool     `json:"useRPCID"`
		RelaysLimit    int      `json:"relaysLimit"`
		StickinessTemp bool     `json:"stickinessTemp"`
		StickyOrigins  []string `json:"stickyOrigins"`
	} `json:"stickinessOptions"`
	Gigastake bool `json:"gigastake"`
}
