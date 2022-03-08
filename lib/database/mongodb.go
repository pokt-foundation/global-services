package database

import (
	"context"

	common "github.com/Pocket/global-dispatcher/common/application"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Mongo struct {
	client   *mongo.Client
	Database string
}

func (m *Mongo) GetAllStakedApplications(ctx context.Context) ([]*common.Application, error) {
	var applications []*common.Application
	collection := m.client.Database(m.Database).Collection("Applications")

	filter := bson.D{
		{
			Key:   "dummy",
			Value: bson.M{"$exists": false},
		},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	for cursor.Next(ctx) {
		var application common.Application
		err := cursor.Decode(&application)
		if err != nil {
			return nil, err
		}

		applications = append(applications, &application)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	cursor.Close(ctx)

	return applications, nil
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
