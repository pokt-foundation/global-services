package pocket

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/Pocket/global-dispatcher/common"
)

// Pocket is a struct containing rpc calls to the Pocket's blockchain network
type Pocket struct {
	Dispatchers []*url.URL
	client      http.Client
}

type GetNetworkApplicationsInput struct {
	AppsPerPage int `json:"per_page"`
	Page        int `json:"page"`
}

type NetworkApplicationsResponse struct {
	Result     []common.NetworkApplication
	TotalPages int `json:"total_pages"`
	Page       int `json:"page"`
}

func NewPocket(dispatchers []string, timeoutSeconds int) (*Pocket, error) {
	var urls []*url.URL

	if len(dispatchers) <= 0 {
		return nil, errors.New("error: a dispatcher URL must be provided")
	}

	for _, dispatcher := range dispatchers {
		u, err := url.Parse(dispatcher)
		if err != nil {
			return nil, err
		}
		urls = append(urls, u)
	}

	return &Pocket{
		Dispatchers: urls,
		client: http.Client{
			Timeout: time.Second * time.Duration(timeoutSeconds),
		},
	}, nil
}

func (p *Pocket) getRandomDispatcher() string {
	return p.Dispatchers[rand.Intn(len(p.Dispatchers))].String()
}

func (p *Pocket) GetNetworkApplications(input GetNetworkApplicationsInput) ([]common.NetworkApplication, error) {
	options := struct {
		Opts GetNetworkApplicationsInput `json:"opts"`
	}{
		Opts: input,
	}
	body, err := json.Marshal(options)
	if err != nil {
		return nil, err
	}
	url := p.getRandomDispatcher() + string(QueryApps)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(string(body)))
	if err != nil {
		return nil, err
	}
	res, err := common.RequestWithRetry(p.client, 3, 0, req)
	if err != nil {
		return nil, err
	}

	var applications *NetworkApplicationsResponse
	err = json.NewDecoder(res.Body).Decode(&applications)
	if err != nil {
		return nil, err
	}

	return applications.Result, nil
}
