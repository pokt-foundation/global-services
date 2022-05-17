package utils

import (
	"io"
	"io/ioutil"
	"net/http"

	pocketUtils "github.com/pokt-foundation/pocket-go/utils"
)

// CloseOrLog ensures that a http connection is safely closed and does not hold file descriptors
func CloseOrLog(response *http.Response) {
	if response != nil {
		// To reuse network connection
		io.Copy(ioutil.Discard, response.Body)
		pocketUtils.CloseOrLog(response.Body)
	}
}
