package gateway

import (
	"encoding/json"

	"github.com/Pocket/global-dispatcher/common/environment"
	httpClient "github.com/Pocket/global-dispatcher/lib/http"
)

var gatewayURL = environment.GetString("GATEWAY_PRODUCTION_URL", "")

const versionPath = "/version"

// GatewayVersion represnts the output from a version call to the pocket's gateway
type GatewayVersion struct {
	Commit string `json:"commit"`
}

func GetGatewayCommitHash() (*GatewayVersion, error) {
	httpClient := *httpClient.NewClient()
	res, err := httpClient.Get(gatewayURL+versionPath, nil)
	if err != nil {
		return nil, err
	}

	var version GatewayVersion
	if err = json.NewDecoder(res.Body).Decode(&version); err != nil {
		return nil, err
	}

	return &version, nil
}
