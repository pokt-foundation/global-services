package metrics

import "time"

// Order of struct fields reflects order of the fields in the db
type Metric struct {
	Timestamp            time.Time
	ApplicationPublicKey string
	Blockchain           string
	NodePublicKey        string
	ElapsedTime          float64
	Bytes                int
	Method               string
	Message              string
	Code                 string
}
