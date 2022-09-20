package models

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// LoadBalancer is the schema of the Load Balancer data
type LoadBalancer struct {
	ID                primitive.ObjectID `json:"_id" bson:"_id"`
	Name              string             `json:"name" bson:"name"`
	ApplicationIDs    []string           `json:"applicationIDs" bson:"applicationIDs"`
	StickinessOptions struct {
		Stickiness    bool     `json:"stickiness" bson:"stickiness"`
		Duration      any      `json:"duration" bson:"duration"`
		UseRPCID      bool     `json:"useRPCID" bson:"useRPCID"`
		RelaysLimit   int      `json:"relaysLimit" bson:"relaysLimit"`
		StickyOrigins []string `json:"stickyOrigins" bson:"stickyOrigins"`
	} `json:"stickinessOptions" bson:"stickinessOptions"`
	Gigastake bool `json:"gigastake" bson:"stickyOrigins"`
}

type LoadBalancerStore interface {
	GetLoadBalancers(ctx context.Context) ([]*LoadBalancer, error)
}
