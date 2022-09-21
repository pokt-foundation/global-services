package postgresgateway

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/pokt-foundation/portal-api-go/repository"
)

type GatewayPostgresRoutes string

const (
	GetAllBlockchains   GatewayPostgresRoutes = "/blockchain"
	GetAllApplications  GatewayPostgresRoutes = "/application"
	GetAllLoadBalancers GatewayPostgresRoutes = "/load_balancer"
)

type Client struct {
	client              *http.Client
	url                 string
	authenticationToken string
}

func NewPostgresClient(url, authenticationToken string) *Client {
	return &Client{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		url:                 url,
		authenticationToken: authenticationToken,
	}
}

func (cl *Client) GetAllLoadBalancers() ([]*repository.LoadBalancer, error) {
	return getAllItems[repository.LoadBalancer](cl.url+string(GetAllLoadBalancers), cl.client, cl.authenticationToken)
}

func (cl *Client) GetAllApplications() ([]*repository.Application, error) {
	return getAllItems[repository.Application](cl.url+string(GetAllApplications), cl.client, cl.authenticationToken)
}

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
