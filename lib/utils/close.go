package utils

import (
	"net/http"

	pocketUtils "github.com/pokt-foundation/pocket-go/pkg/utils"
)

func CloseOrLog(response *http.Response) {
	if response != nil {
		pocketUtils.CloseOrLog(response.Body)
	}
}
