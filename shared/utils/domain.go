package utils

import (
	"fmt"
	_url "net/url"
	"strings"
)

// GetDomainFromURL returns only the domain given an URL address, if the string
// given is not a valid URL then returns the URL unchanged.
func GetDomainFromURL(url string) string {
	u, err := _url.Parse(url)
	if err != nil {
		return url
	}

	parts := strings.Split(u.Hostname(), ".")

	if len(parts) < 2 {
		return url
	}

	domain := parts[len(parts)-2] + "." + parts[len(parts)-1]
	fmt.Println("AFTER PARTS")

	return domain
}
