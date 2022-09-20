package database

import (
	"context"

	"github.com/Pocket/global-services/shared/gateway/models"
	"github.com/Pocket/global-services/shared/logger"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Mongo Represents a mongo client and gateway related operations
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

// GetAppsFromList takes a list of app ids and returns the collections of these.
func (m *Mongo) GetAppsFromList(ctx context.Context, appIDs []string) ([]*models.Application, error) {
	var applicationIDs []*primitive.ObjectID
	for _, appID := range appIDs {
		objectID, err := primitive.ObjectIDFromHex(appID)
		if err != nil {
			logger.Log.WithFields(logrus.Fields{
				"typeID": appID,
				"error":  err.Error(),
			}).Warn("error converting from string to Object id: " + err.Error())
		}
		applicationIDs = append(applicationIDs, &objectID)
	}

	return filterCollection[models.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "_id",
			Value: bson.M{"$in": applicationIDs},
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

	var applicationIDs []string
	for _, lb := range loadBalancers {
		applicationIDs = append(applicationIDs, lb.ApplicationIDs...)
	}

	return m.GetAppsFromList(ctx, applicationIDs)
}

// GetBlockchains returns the blockchains on the db
func (m *Mongo) GetBlockchains(ctx context.Context) ([]*models.Blockchain, error) {
	return filterCollection[models.Blockchain](ctx, *m.client, m.Database, "Blockchains", bson.D{})
}

// GetBlockchains returns the applications on the db
func (m *Mongo) GetApplications(ctx context.Context) ([]*models.Application, error) {
	return filterCollection[models.Application](ctx, *m.client, m.Database, "Applications", bson.D{})
}

// GetBlockchains returns the load balancers on the db
func (m *Mongo) GetLoadBalancers(ctx context.Context) ([]*models.LoadBalancer, error) {
	return filterCollection[models.LoadBalancer](ctx, *m.client, m.Database, "LoadBalancers", bson.D{})
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

// InsertOneCollection inserts one document to the specified collection
func (m *Mongo) InsertOneCollection(ctx context.Context, collectionName string, document any) error {
	collection := m.client.Database(m.Database).Collection(collectionName)
	_, err := collection.InsertOne(ctx, document)
	return err
}

// InsertManyCollection inserts N document to the specified collection
func (m *Mongo) InsertManyCollections(ctx context.Context, collectionName string, documents []any) error {
	collection := m.client.Database(m.Database).Collection(collectionName)
	_, err := collection.InsertMany(ctx, documents)
	return err
}
