package cpicker

import "context"

// Session model of the aggregated data of cherry picker of all regions
type Session struct {
	PublicKey            string  `json:"publicKey"`
	Chain                string  `json:"chain"`
	SessionKey           string  `json:"sessionKey"`
	Address              string  `json:"address"`
	ApplicationPublicKey string  `json:"applicationPublicKey"`
	SessionHeight        int     `json:"sessionHeight"`
	TotalSuccess         int     `json:"totalSuccess"`
	TotalFailure         int     `json:"totalFailure"`
	AverageSuccessTime   float64 `json:"averageSuccessTime"`
	Failure              bool    `json:"failure"`
}

// Region model of data of cherry picker for a single region
type Region struct {
	PublicKey                 string    `json:"publicKey"`
	Chain                     string    `json:"chain"`
	SessionKey                string    `json:"sessionKey"`
	Region                    string    `json:"region"`
	Address                   string    `json:"address"`
	ApplicationPublicKey      string    `json:"applicationPublicKey"`
	SessionHeight             int       `json:"sessionHeight"`
	TotalSuccess              int       `json:"totalSuccess"`
	TotalFailure              int       `json:"totalFailure"`
	MedianSuccessLatency      []float32 `json:"medianSuccessLatency"`
	WeightedSuccessLatency    []float32 `json:"weightedSuccessLatency"`
	P90Latency                []float32 `json:"p90Latency"`
	SuccessRate               []float32 `json:"successRate"`
	Attempts                  []int     `json:"attempts"`
	AvgSuccessLatency         float32   `json:"avgSuccessLatency"`
	AvgWeightedSuccessLatency float32   `json:"avgWeightedSuccessLatency"`
	Failure                   bool      `json:"failure"`
}

// SessionUpdatePayload payload to update a session
type SessionUpdatePayload struct {
	PublicKey          string  `json:"publicKey"`
	Chain              string  `json:"chain"`
	SessionKey         string  `json:"sessionKey"`
	TotalSuccess       int     `json:"totalSuccess"`
	TotalFailure       int     `json:"totalFailure"`
	AverageSuccessTime float32 `json:"averageSuccessTime"`
	Failure            bool    `json:"failure"`
}

// RegionUpdatePayload payload to update a region
type RegionUpdatePayload struct {
	PublicKey                 string  `json:"publicKey"`
	Chain                     string  `json:"chain"`
	SessionKey                string  `json:"sessionKey"`
	Region                    string  `json:"region"`
	TotalSuccess              int     `json:"totalSuccess"`
	TotalFailure              int     `json:"totalFailure"`
	Attempts                  int     `json:"attempts"`
	MedianSuccessLatency      float32 `json:"medianSuccessLatency"`
	WeightedSuccessLatency    float32 `json:"weightedSuccessLatency"`
	AvgSuccessLatency         float32 `json:"avgSuccessLatency"`
	AvgWeightedSuccessLatency float32 `json:"avgWeightedSuccessLatency"`
	P90Latency                float32 `json:"p90Latency"`
	SuccessRate               float32 `json:"successRate"`
	Failure                   bool    `json:"failure"`
}

// ServiceLog represents a snapshot of a node's performance in the portal-api
type ServiceLog struct {
	Results                map[string]int `json:"results"`
	MedianSuccessLatency   string         `json:"medianSuccessLatency"`
	WeightedSuccessLatency string         `json:"weightedSuccessLatency"`
	SessionKey             string         `json:"sessionKey"`
	SessionHeight          int            `json:"sessionHeight"`
	Metadata               struct {
		P90                  float32 `json:"p90"`
		Attempts             int     `json:"attempts"`
		SuccessRate          float32 `json:"successRate"`
		ApplicationPublicKey string  `json:"applicationPublicKey"`
	} `json:"metadata"`
}

// CherryPickerStore is the interface for all the operations on the cherry picker model
type CherryPickerStore interface {
	GetSession(ctx context.Context, publicKey, chain, sessionKey string) (*Session, error)
	CreateSession(ctx context.Context, session *Session) error
	UpdateSession(ctx context.Context, session *SessionUpdatePayload) (*Session, error)
	GetSessionRegions(ctx context.Context, publicKey, chain, sessionKey string) ([]*Region, error)
	GetRegion(ctx context.Context, publicKey, chain, sessionKey, region string) (*Region, error)
	CreateRegion(ctx context.Context, region *Region) error
	UpdateRegion(ctx context.Context, region *RegionUpdatePayload) (*Region, error)
	GetConnection() string
}
