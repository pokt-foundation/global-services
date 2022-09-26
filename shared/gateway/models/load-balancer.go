package models

import (
	"context"
	"strconv"

	"github.com/pokt-foundation/portal-api-go/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LoadBalancer is the schema of the Load Balancer data
type LoadBalancer struct {
	ID                primitive.ObjectID `json:"_id" bson:"_id"`
	Name              string             `json:"name" bson:"name"`
	User              string             `json:"user" bson:"user"`
	ApplicationIDs    []string           `json:"applicationIDs" bson:"applicationIDs"`
	StickinessOptions struct {
		Stickiness    bool     `json:"stickiness" bson:"stickiness"`
		Duration      any      `json:"duration" bson:"duration"`
		UseRPCID      bool     `json:"useRPCID" bson:"useRPCID"`
		RelaysLimit   int      `json:"relaysLimit" bson:"relaysLimit"`
		StickyOrigins []string `json:"stickyOrigins" bson:"stickyOrigins"`
	} `json:"stickinessOptions" bson:"stickinessOptions"`
	Gigastake         bool `json:"gigastake" bson:"gigastake"`
	GigastakeRedirect bool `json:"gigastakeRedirect" bson:"gigastakeRedirect"`
	// Value is currently set in both string and int forms in the database
	RequestTimeout any `json:"requestTimeout" bson:"requestTimeout"`
}

// RepositoryToModelLoadBalancer converts the migrated Load Balancer struct to one of mongodb
func RepositoryToModelLoadBalancer(lb *repository.LoadBalancer) (*LoadBalancer, error) {
	id := primitive.NewObjectID()
	var err error
	if lb.ID != "" {
		id, err = primitive.ObjectIDFromHex(lb.ID)
		if err != nil {
			return nil, err
		}
	}

	appIDs := make([]string, len(lb.ApplicationIDs))
	for _, app := range lb.Applications {
		appIDs = append(appIDs, app.ID)
	}

	return &LoadBalancer{
		ID:                id,
		Name:              lb.Name,
		User:              lb.UserID,
		ApplicationIDs:    appIDs,
		Gigastake:         lb.Gigastake,
		GigastakeRedirect: lb.GigastakeRedirect,
		RequestTimeout:    strconv.Itoa(lb.RequestTimeout),
	}, nil
}

// LoadBalancerStore is the interface for all the operations to retrieve data of load balancers
type LoadBalancerStore interface {
	GetLoadBalancers(ctx context.Context) ([]*LoadBalancer, error)
}
