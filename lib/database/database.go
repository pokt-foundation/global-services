package database

import (
	"context"
	"fmt"

	common "github.com/Pocket/global-dispatcher/common/application"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	client   *mongo.Client
	Database string
}

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

func (m *Mongo) GetAllStakedApplications(ctx context.Context) ([]*common.Application, error) {
	return filterCollection[common.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "dummy",
			Value: bson.M{"$exists": false},
		},
	})
}

func (m *Mongo) GetAllGigastakedApplications(ctx context.Context) ([]*common.Application, error) {
	loadBalancers, err := filterCollection[common.LoadBalancer](ctx, *m.client, m.Database, "LoadBalancers", bson.D{
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

	return filterCollection[common.Application](ctx, *m.client, m.Database, "Applications", bson.D{
		{
			Key:   "_id",
			Value: bson.M{"$in": applicationIDs},
		},
	})
}

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
