package metrics

import (
	"context"

	logger "github.com/Pocket/global-dispatcher/lib/logger"
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

func NewMetricsRecorder(ctx context.Context, connectionString string) (*Recorder, error) {
	conn, err := pgxpool.Connect(ctx, connectionString)
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
