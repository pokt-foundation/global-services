package database

import (
	"context"
	"fmt"

	"github.com/Pocket/global-dispatcher/common/gateway"
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

// GetStakedApplications returns all the collections that are staked on the db
func (m *Mongo) GetStakedApplications(ctx context.Context) ([]*gateway.Application, error) {
	return filterCollection[gateway.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "dummy",
			Value: bson.M{"$exists": false},
		},
	})
}

// GetSettlersApplications returns only the applications marked as 'Settlers'
func (m *Mongo) GetSettlersApplications(ctx context.Context) ([]*gateway.Application, error) {
	return filterCollection[gateway.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key: "name",
			Value: bson.M{"$regex": primitive.Regex{
				Pattern: "Settlers",
				Options: "im",
			}},
		},
	})
}

// GetGigastakedApplications returns all the applications that belong to a
// gigastake load balancer
func (m *Mongo) GetGigastakedApplications(ctx context.Context) ([]*gateway.Application, error) {
	loadBalancers, err := filterCollection[gateway.LoadBalancer](ctx, *m.client, m.Database, "LoadBalancers", bson.D{
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
				fmt.Println("error converting from string to Object id: " + err.Error())
			}
			applicationIDs = append(applicationIDs, &objectID)
		}
	}

	return filterCollection[gateway.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "_id",
			Value: bson.M{"$in": applicationIDs},
		},
	})
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
