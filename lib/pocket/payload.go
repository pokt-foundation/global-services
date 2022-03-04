package pocket

type GetNetworkApplicationsInput struct {
	AppsPerPage int `json:"per_page"`
	Page        int `json:"page"`
}

type GetNetworkApplicationsOutput struct {
	Result     []NetworkApplication
	TotalPages int `json:"total_pages"`
	Page       int `json:"page"`
}

type DispatchInput struct {
	AppPublicKey  string `json:"app_public_key"`
	Chain         string `json:"chain"`
	SessionHeight int    `json:"session_height"`
}

type DispatchOutput struct {
	BlockHeight int     `json:"block_height"`
	Session     Session `json:"session"`
}
