package postgresclient

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

type GatewayPostgres struct {
	client              *http.Client
	url                 string
	authenticationToken string
}

func NewPostgresClient(url, authenticationToken string) *GatewayPostgres {
	return &GatewayPostgres{
		client: &http.Client{
			Timeout: 20 * time.Second,
		},
		url:                 url,
		authenticationToken: authenticationToken,
	}
}

func (gp *GatewayPostgres) GetAllLoadBalancers() ([]repository.LoadBalancer, error) {
	return getAllItems[repository.LoadBalancer](gp.url+string(GetAllLoadBalancers), gp.client, gp.authenticationToken)
}

func (gp *GatewayPostgres) GetAllApplications() ([]repository.Application, error) {
	return getAllItems[repository.Application](gp.url+string(GetAllApplications), gp.client, gp.authenticationToken)
}

func (gp *GatewayPostgres) GetAllBlockchains() ([]repository.Blockchain, error) {
	return getAllItems[repository.Blockchain](gp.url+string(GetAllBlockchains), gp.client, gp.authenticationToken)
}

func getAllItems[T any](path string, client *http.Client, authToken string) ([]T, error) {
	var items []T

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
