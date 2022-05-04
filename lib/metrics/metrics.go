package metrics

import (
	"context"
	"fmt"

	"github.com/Pocket/global-services/lib/logger"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type Recorder struct {
	Conn *pgxpool.Pool
}

type MetricData struct {
	Metric    *Metric
	RequestID string
	TypeID    string
}

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

func (mr *Recorder) WriteErrorMetric(ctx context.Context, metric *MetricData) {
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
		metric.Metric.Timestamp,
		metric.Metric.ApplicationPublicKey,
		metric.Metric.Blockchain,
		metric.Metric.NodePublicKey,
		metric.Metric.ElapsedTime,
		metric.Metric.Bytes,
		metric.Metric.Method,
		metric.Metric.Message,
		metric.Metric.Code)

	if err != nil {
		logger.Log.WithFields(log.Fields{
			"requestID":    metric.RequestID,
			"serviceNode":  metric.Metric.NodePublicKey,
			"appPublicKey": metric.TypeID,
		}).Error("metrics: failure recording metric: " + err.Error())
	}
}
