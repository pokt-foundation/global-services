package utils

import (
	"io"
	"io/ioutil"
	"net/http"

	pocketUtils "github.com/pokt-foundation/pocket-go/pkg/utils"
)

func CloseOrLog(response *http.Response) {
	if response != nil {
		// To reuse network connection
		io.Copy(ioutil.Discard, response.Body)
		pocketUtils.CloseOrLog(response.Body)
	}
}
