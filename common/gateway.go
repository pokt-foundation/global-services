package gateway

const VersionPath = "/version"

// Version represnts the output from a version call to the portal-api
type Version struct {
	Commit string `json:"commit"`
}
