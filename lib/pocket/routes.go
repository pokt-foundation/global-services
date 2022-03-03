package pocket

type Versions string

const (
	V1 Versions = "/v1"
)

type V1RPCRoutes string

const (
	QueryApps V1RPCRoutes = V1RPCRoutes(V1) + "/query/apps"
)
