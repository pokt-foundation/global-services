package metrics

import (
	"context"

	"github.com/Pocket/global-services/shared/database"
	"github.com/Pocket/global-services/shared/logger"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

// Recorder represents the struct to write records to the metrics database
type Recorder struct {
	conn *pgxpool.Pool
}

// MetricsData Represents the

// NewMetricsRecorder returns a connection to a metrics database
func NewMetricsRecorder(ctx context.Context, options *database.PostgresOptions) (*Recorder, error) {
	postgres, err := database.NewPostgresDatabase(ctx, options)
	if err != nil {
		return nil, errors.New("unable to connect to metrics db: " + err.Error())
	}

	return &Recorder{
		conn: postgres.Conn,
	}, nil
}

// WriteErrorMetric writes an error metric to the connected database
func (r *Recorder) WriteErrorMetric(ctx context.Context, metric *Metric) {
	_, err := r.conn.Exec(ctx, `
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

// Close closes the postgres connection of the metrics db
func (r *Recorder) Close() {
	r.conn.Close()
}
