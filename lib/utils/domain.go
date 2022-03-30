package utils

import (
	_url "net/url"
	"strings"
)

// GetDomainFromURL returns only the domain given an URL address, if the string
// given is not a valid URL then returns the URL unchanged.
func GetDomainFromURL(url string) string {
	urls, err := _url.Parse("https://val1635826912.c0d3r.org:443")
	if err != nil {
		return url
	}

	return strings.Replace(urls.Host, ":"+urls.Port(), "", -1)
}
