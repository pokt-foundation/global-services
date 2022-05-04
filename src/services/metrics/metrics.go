package metrics

import (
	"context"
	"fmt"

	"github.com/Pocket/global-services/src/services/logger"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Recorder represents the struct to write records to the metrics database
type Recorder struct {
	Conn *pgxpool.Pool
}

// MetricsData Represents the

// NewMetricsRecorder returns a connection to a metrics database
func NewMetricsRecorder(ctx context.Context, connectionString string, minPoolSize, maxPoolSize int) (*Recorder, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf("%s?pool_min_conns=%d&pool_max_conns=%d", connectionString, minPoolSize, maxPoolSize))
	if err != nil {
		return nil, errors.New("unable to connect to database" + err.Error())
	}

	conn, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, errors.New("unable to connect to database: " + err.Error())
	}

	return &Recorder{
		Conn: conn,
	}, nil
}

// WriteErrorMetric writes an error metric to the connected database
func (mr *Recorder) WriteErrorMetric(ctx context.Context, metric *Metric) {
	_, err := mr.Conn.Exec(ctx, `
	INSERT INTO
	 error
	 (timestamp,
		applicationpublickey,
		blockchain,
		nodepublickey,
		elapsedTime,
		bytes,
		method,
		message,
		code
		)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		metric.Timestamp,
		metric.ApplicationPublicKey,
		metric.Blockchain,
		metric.NodePublicKey,
		metric.ElapsedTime,
		metric.Bytes,
		metric.Method,
		metric.Message,
		metric.Code)

	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":    metric.RequestID,
			"serviceNode":  metric.NodePublicKey,
			"appPublicKey": metric.TypeID,
		}).Error("metrics: failure recording metric: " + err.Error())
	}
}
