package cpicker

// CherryPickerSession model of the aggregated data of cherry picker of all regions
type CherryPickerSession struct {
	PublicKey          string  `json:"publicKey"`
	Chain              string  `json:"chain"`
	SessionKey         string  `json:"sessionKey"`
	SessionHeight      string  `json:"sessionHeight"`
	Address            string  `json:"address"`
	AggregateSuccesses float32 `json:"aggregateSuccesses"`
	AggregateFailures  float32 `json:"aggregateFailures"`
	Failure            bool    `json:"failure"`
}

// CherryPickerSessionRegion model of data of cherry picker for a single region
type CherryPickerSessionRegion struct {
	PublicKey            string    `json:"publicKey"`
	Chain                string    `json:"chain"`
	SessionKey           string    `json:"sessionKey"`
	Region               string    `json:"region"`
	MedianSuccessLatency []float32 `json:"medianSuccessLatency"`
	AggregateSuccesses   int       `json:"aggregateSuccesses"`
	AggregateFailures    int       `json:"aggregateFailures"`
}

// CherryPickerStore is the interface for all the operations on the cherry picker model
type CherryPickerStore interface {
}

type ServiceLog struct {
	Results                map[string]int `json:"results"`
	MedianSuccessLatency   string         `json:"medianSuccessLatency"`
	WeightedSuccessLatency string         `json:"weightedSuccessLatency"`
	SessionKey             string         `json:"string"`
}
