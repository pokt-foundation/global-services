package database

import (
	"context"

	"github.com/Pocket/global-services/common/gateway/models"
	"github.com/Pocket/global-services/lib/logger"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	client   *mongo.Client
	Database string
}

// ClientFromURI returns a mongodb client from an URI
func ClientFromURI(ctx context.Context, uri string, database string) (*Mongo, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))

	if err != nil {
		return nil, err
	}

	return &Mongo{
		client:   client,
		Database: database,
	}, nil
}

// GetStakedApplications returns the applications that are staked on the db
func (m *Mongo) GetStakedApplications(ctx context.Context) ([]*models.Application, error) {
	return filterCollection[models.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "dummy",
			Value: bson.M{"$exists": false},
		},
	})
}

// GetSettlersApplications returns only the applications marked as 'Settlers'
func (m *Mongo) GetSettlersApplications(ctx context.Context) ([]*models.Application, error) {
	return filterCollection[models.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key: "name",
			Value: bson.M{"$regex": primitive.Regex{
				Pattern: "Settlers",
				Options: "im",
			}},
		},
	})
}

// GetGigastakedApplications returns the applications that belong to a
// gigastake load balancer
func (m *Mongo) GetGigastakedApplications(ctx context.Context) ([]*models.Application, error) {
	loadBalancers, err := filterCollection[models.LoadBalancer](ctx, *m.client, m.Database, "LoadBalancers", bson.D{
		{
			Key:   "gigastake",
			Value: true,
		},
	})
	if err != nil {
		return nil, err
	}

	var applicationIDs []*primitive.ObjectID
	for _, lb := range loadBalancers {
		for _, appID := range lb.ApplicationIDs {
			objectID, err := primitive.ObjectIDFromHex(appID)
			if err != nil {
				logger.Log.WithFields(logrus.Fields{
					"typeID": appID,
					"error":  err.Error(),
				}).Warn("error converting from string to Object id: " + err.Error())
			}
			applicationIDs = append(applicationIDs, &objectID)
		}
	}

	return filterCollection[models.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "_id",
			Value: bson.M{"$in": applicationIDs},
		},
	})
}

// GetBlockchains returns the blockchains on the db
func (m *Mongo) GetBlockchains(ctx context.Context) ([]*models.Blockchain, error) {
	return filterCollection[models.Blockchain](ctx, *m.client, m.Database, "Blockchains", bson.D{})
}

// filterCollection returns a collection marshalled to a struct given the filter
func filterCollection[T any](ctx context.Context, client mongo.Client, database, collectionName string, filter primitive.D) ([]*T, error) {
	documents := []*T{}
	collection := client.Database(database).Collection(collectionName)

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	for cursor.Next(ctx) {
		var document T
		err := cursor.Decode(&document)
		if err != nil {
			return nil, err
		}

		documents = append(documents, &document)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	cursor.Close(ctx)

	return documents, nil
}
