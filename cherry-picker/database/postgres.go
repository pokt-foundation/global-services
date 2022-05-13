package database

import (
	"context"
	"errors"
	"fmt"
	"strings"

	cpicker "github.com/Pocket/global-services/cherry-picker"

	"github.com/Pocket/global-services/shared/database"
)

var (
	// ErrEmptySessionTableName when session table name is missing
	ErrEmptySessionTableName = errors.New("session table name is empty")
	// ErrEmptySessionRegionTableName when session region table name is missing
	ErrEmptySessionRegionTableName = errors.New("session region table name is empty")
	// ErrNotFound when no rows are found
	ErrNotFound = errors.New("no rows in result set")
	// ErrDuplicate when trying to insert a value that already exist
	ErrDuplicate = errors.New("duplicate key value violates unique constraint")
)

// CherryPickerDB is an interface to operations in the cherry picker database
type CherryPickerPostgres struct {
	Db                     *database.Postgres
	SessionTableName       string
	SessionRegionTableName string
}

func NewCherryPickerPostgresFromConnectionString(ctx context.Context, options *database.PostgresOptions, sessionTableName, sessionRegionTableName string) (*CherryPickerPostgres, error) {
	if sessionTableName == "" {
		return nil, ErrEmptySessionTableName
	}

	if sessionRegionTableName == "" {
		return nil, ErrEmptySessionRegionTableName
	}

	db, err := database.NewPostgresDatabase(ctx, options)
	if err != nil {
		return nil, errors.New("unable to connect to metrics db: " + err.Error())
	}

	return &CherryPickerPostgres{
		Db:                     db,
		SessionTableName:       sessionTableName,
		SessionRegionTableName: sessionRegionTableName,
	}, nil
}

func (ch *CherryPickerPostgres) GetSession(ctx context.Context, publicKey, chain, sessionKey string) (*cpicker.Session, error) {
	var session cpicker.Session

	err := ch.Db.Conn.QueryRow(context.Background(), fmt.Sprintf(`
	SELECT *
	FROM %s
	WHERE public_key = $1
	  AND chain = $2
	  AND session_key = $3
	`, ch.SessionTableName), publicKey, chain, sessionKey).Scan(
		&session.PublicKey,
		&session.Chain,
		&session.SessionKey,
		&session.SessionHeight,
		&session.Address,
		&session.TotalSuccess,
		&session.TotalFailure,
		&session.AverageSuccessTime,
		&session.Failure)
	if err != nil {
		return nil, getCustomError(err)
	}
	return &session, nil
}

func (ch *CherryPickerPostgres) CreateSession(ctx context.Context, session *cpicker.Session) error {
	_, err := ch.Db.Conn.Exec(ctx, fmt.Sprintf(`
	INSERT INTO
	 %s
	 (public_key,
		chain,
		session_key,
		session_height,
		address,
		total_success,
		total_failure,
		avg_success_time,
		failure
		)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`, ch.SessionTableName),
		session.PublicKey,
		session.Chain,
		session.SessionKey,
		session.SessionHeight,
		session.Address,
		session.TotalSuccess,
		session.TotalFailure,
		session.AverageSuccessTime,
		session.Failure)

	return getCustomError(err)
}

func (ch *CherryPickerPostgres) UpdateSession(ctx context.Context, session *cpicker.UpdateSession) (*cpicker.Session, error) {
	var updatedSession cpicker.Session

	err := ch.Db.Conn.QueryRow(ctx, fmt.Sprintf(`
	UPDATE %s
	SET total_success = $1,
		total_failure = $2,
		avg_success_time = $3,
		failure = $4
	WHERE public_key = $5
		AND chain = $6
		AND session_key = $7
	RETURNING *`,
		ch.SessionTableName),
		session.TotalSuccess,
		session.TotalFailure,
		session.AverageSuccessTime,
		session.Failure,
		session.PublicKey,
		session.Chain,
		session.SessionKey).Scan(
		&updatedSession.PublicKey,
		&updatedSession.Chain,
		&updatedSession.SessionKey,
		&updatedSession.SessionHeight,
		&updatedSession.Address,
		&updatedSession.TotalSuccess,
		&updatedSession.TotalFailure,
		&updatedSession.AverageSuccessTime,
		&updatedSession.Failure)

	return &updatedSession, getCustomError(err)
}

func (ch *CherryPickerPostgres) GetSessionRegions(ctx context.Context, publicKey, chain, sessionKey string) ([]*cpicker.Region, error) {
	regions := []*cpicker.Region{}

	rows, err := ch.Db.Conn.Query(ctx, fmt.Sprintf(`
	SELECT *
	FROM % s
	WHERE public_key = $1
		AND chain = $2
		AND session_key = $3`, ch.SessionRegionTableName), publicKey, chain, sessionKey)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var region cpicker.Region
		if err := rows.Scan(
			&region.PublicKey,
			&region.Chain,
			&region.SessionKey,
			&region.SessionHeight,
			&region.Region,
			&region.Address,
			&region.TotalSuccess,
			&region.TotalFailure,
			&region.MedianSuccessLatency,
			&region.WeightedSuccessLatency,
			&region.AvgSuccessLatency,
			&region.AvgWeightedSuccessLatency,
			&region.Failure); err != nil {
			return nil, err
		}
		regions = append(regions, &region)
	}

	return regions, nil
}

func (ch *CherryPickerPostgres) GetRegion(ctx context.Context, publicKey, chain, sessionKey, region string) (*cpicker.Region, error) {
	var sessionRegion cpicker.Region

	err := ch.Db.Conn.QueryRow(context.Background(), fmt.Sprintf(`
	SELECT * FROM %s WHERE 
		public_key = $1 AND
		chain = $2 AND
		session_key = $3 AND
		region = $4
	`, ch.SessionRegionTableName), publicKey, chain, sessionKey, region).Scan(
		&sessionRegion.PublicKey,
		&sessionRegion.Chain,
		&sessionRegion.SessionKey,
		&sessionRegion.SessionHeight,
		&sessionRegion.Region,
		&sessionRegion.Address,
		&sessionRegion.TotalSuccess,
		&sessionRegion.TotalFailure,
		&sessionRegion.MedianSuccessLatency,
		&sessionRegion.WeightedSuccessLatency,
		&sessionRegion.AvgSuccessLatency,
		&sessionRegion.AvgWeightedSuccessLatency,
		&sessionRegion.Failure)
	if err != nil {
		return nil, getCustomError(err)
	}
	return &sessionRegion, nil
}

func (ch *CherryPickerPostgres) CreateRegion(ctx context.Context, region *cpicker.Region) error {
	_, err := ch.Db.Conn.Exec(ctx, fmt.Sprintf(`
	INSERT INTO
	 %s
	 (public_key,
		chain,
		session_key,
		region,
		session_height,
		address,
		total_success,
		total_failure,
		median_success_latency,
		weighted_success_latency,
		avg_success_latency,
		avg_weighted_success_latency,
		failure
		)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, $10, $11, $12, $13)`, ch.SessionRegionTableName),
		region.PublicKey,
		region.Chain,
		region.SessionKey,
		region.Region,
		region.SessionHeight,
		region.Address,
		region.TotalSuccess,
		region.TotalFailure,
		region.MedianSuccessLatency,
		region.WeightedSuccessLatency,
		region.AvgSuccessLatency,
		region.AvgWeightedSuccessLatency,
		region.Failure)

	return getCustomError(err)
}

func (ch *CherryPickerPostgres) UpdateRegion(ctx context.Context, region *cpicker.UpdateRegion) (*cpicker.Region, error) {
	var updatedSessionRegion cpicker.Region

	err := ch.Db.Conn.QueryRow(ctx, fmt.Sprintf(`
	UPDATE %s
	SET total_success = $1,
		total_failure = $2,
		median_success_latency = array_append(median_success_latency, $3),
		weighted_success_latency = array_append(weighted_success_latency, $4),
		avg_success_latency = $5,
		avg_weighted_success_latency = $6,
		failure = $7
	WHERE public_key = $8
		AND chain = $9
		AND session_key = $10
		AND region = $11
	RETURNING *`,
		ch.SessionRegionTableName),
		region.TotalSuccess,
		region.TotalFailure,
		region.MedianSuccessLatency,
		region.WeightedSuccessLatency,
		region.AvgSuccessLatency,
		region.AvgWeightedSuccessLatency,
		region.Failure,
		region.PublicKey,
		region.Chain,
		region.SessionKey,
		region.Region).Scan(
		&updatedSessionRegion.PublicKey,
		&updatedSessionRegion.Chain,
		&updatedSessionRegion.SessionKey,
		&updatedSessionRegion.SessionHeight,
		&updatedSessionRegion.Region,
		&updatedSessionRegion.Address,
		&updatedSessionRegion.TotalSuccess,
		&updatedSessionRegion.TotalFailure,
		&updatedSessionRegion.MedianSuccessLatency,
		&updatedSessionRegion.WeightedSuccessLatency,
		&updatedSessionRegion.AvgSuccessLatency,
		&updatedSessionRegion.AvgWeightedSuccessLatency,
		&updatedSessionRegion.Failure)

	return &updatedSessionRegion, getCustomError(err)
}

func getCustomError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case err.Error() == ErrNotFound.Error():
		return ErrNotFound

	case strings.Contains(err.Error(), ErrDuplicate.Error()):
		return ErrDuplicate
	}
	return err
}