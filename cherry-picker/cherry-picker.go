package cpicker

import "context"

// Session model of the aggregated data of cherry picker of all regions
type Session struct {
	PublicKey          string  `json:"publicKey"`
	Chain              string  `json:"chain"`
	SessionKey         string  `json:"sessionKey"`
	Address            string  `json:"address"`
	SessionHeight      int     `json:"sessionHeight"`
	TotalSuccess       int     `json:"totalSuccess"`
	TotalFailure       int     `json:"totalFailure"`
	AverageSuccessTime float64 `json:"averageSuccessTime"`
	Failure            bool    `json:"failure"`
}

// SessionRegion model of data of cherry picker for a single region
type SessionRegion struct {
	PublicKey                 string    `json:"publicKey"`
	Chain                     string    `json:"chain"`
	SessionKey                string    `json:"sessionKey"`
	Address                   string    `json:"address"`
	Region                    string    `json:"region"`
	SessionHeight             int       `json:"sessionHeight"`
	TotalSuccess              int       `json:"aggregateSuccesses"`
	TotalFailure              int       `json:"aggregateFailures"`
	MedianSuccessLatency      []float32 `json:"medianSuccessLatency"`
	WeightedSuccessLatency    []float32 `json:"weightedSuccessLatency"`
	AvgSuccessLatency         float32   `json:"avgSuccessLatency"`
	AvgWeightedSuccessLatency float32   `json:"avgWeightedSuccessLatency"`
	Failure                   bool      `json:"failure"`
}

// ServiceLog represents a snapshot of a node's performance in the portal-api
type ServiceLog struct {
	Results                map[string]int `json:"results"`
	MedianSuccessLatency   string         `json:"medianSuccessLatency"`
	WeightedSuccessLatency string         `json:"weightedSuccessLatency"`
	SessionKey             string         `json:"sessionKey"`
	SessionHeight          int            `json:"sessionHeight"`
}

// CherryPickerStore is the interface for all the operations on the cherry picker model
type CherryPickerStore interface {
	GetSession(ctx context.Context, publicKey, chain, sessionKey string) (*Session, error)
	CreateSession(ctx context.Context, session *Session) error
	UpdateSession(ctx context.Context, session *Session) error
	GetSessionRegion(ctx context.Context, publicKey, chain, sessionKey, region string) (*SessionRegion, error)
	CreateSessionRegion(ctx context.Context, sessionRegion *SessionRegion) error
	UpdateSessionRegion(ctx context.Context, sessionRegion *SessionRegion) error
}
