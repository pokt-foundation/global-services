package gateway

import (
	"encoding/json"

	"github.com/Pocket/global-dispatcher/common/environment"
	httpClient "github.com/Pocket/global-dispatcher/lib/http"
)

var gatewayURL = environment.GetString("GATEWAY_PRODUCTION_URL", "")

const versionPath = "/version"

func GetGatewayCommitHash() (string, error) {
	httpClient := *httpClient.NewClient()
	res, err := httpClient.Get(gatewayURL+versionPath, nil)
	if err != nil {
		return "", err
	}

	var commitHash struct {
		Commit string `json:"commit"`
	}

	return commitHash.Commit, json.NewDecoder(res.Body).Decode(&commitHash)
}
