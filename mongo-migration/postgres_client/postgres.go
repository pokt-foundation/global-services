package postgresgateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pokt-foundation/portal-api-go/repository"
)

// GatewayPostgresRoutes holds the routes to obtain data from the postgrses client
type GatewayPostgresRoutes string

const (
	// GetAllBlockchains path to get all blockchains
	GetAllBlockchains GatewayPostgresRoutes = "/blockchain"
	// GetAllApplications path to get all applications
	GetAllApplications GatewayPostgresRoutes = "/application"
	// GetAllLoadBalancers path to get all load balancers
	GetAllLoadBalancers GatewayPostgresRoutes = "/load_balancer"
)

// Client is the struct to communicate with the postgres database
type Client struct {
	client              *http.Client
	url                 string
	authenticationToken string
}

// NewPostgresClient returns a new postgres clients given the credentials
func NewPostgresClient(url, authenticationToken string) *Client {
	return &Client{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		url:                 url,
		authenticationToken: authenticationToken,
	}
}

// GetAllLoadBalancers retrieves all the load balancers from the database
func (cl *Client) GetAllLoadBalancers() ([]*repository.LoadBalancer, error) {
	return getAllItems[repository.LoadBalancer](cl.url+string(GetAllLoadBalancers), cl.client, cl.authenticationToken)
}

// GetAllApplications retrieves all the applications from the database
func (cl *Client) GetAllApplications() ([]*repository.Application, error) {
	return getAllItems[repository.Application](cl.url+string(GetAllApplications), cl.client, cl.authenticationToken)
}

// GetAllBlockchains retrieves all the blockchains from the database
func (cl *Client) GetAllBlockchains() ([]*repository.Blockchain, error) {
	return getAllItems[repository.Blockchain](cl.url+string(GetAllBlockchains), cl.client, cl.authenticationToken)
}

func getAllItems[T any](path string, client *http.Client, authToken string) ([]*T, error) {
	var items []*T

	req, err := http.NewRequest(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authToken)

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return items, json.NewDecoder(res.Body).Decode(&items)
}
