package pocket

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/url"

	"github.com/Pocket/global-dispatcher/common"
	httpClient "github.com/Pocket/global-dispatcher/lib/http"
)

// Pocket is a struct containing rpc calls to the Pocket's blockchain network
type Pocket struct {
	RPCProvider *url.URL
	Dispatchers []*url.URL
	client      httpClient.Client
}

type performRequestOptions struct {
	route  V1RPCRoutes
	rpcURL string
	body   interface{}
}

func NewPocketClient(httpRpcURL string, dispatchers []string, timeoutSeconds int) (*Pocket, error) {
	var dispatcherURLs []*url.URL

	if len(dispatchers) <= 0 {
		return nil, errors.New("error: a dispatcher URL must be provided")
	}

	for _, dispatcher := range dispatchers {
		u, err := url.Parse(dispatcher)
		if err != nil {
			return nil, err
		}
		dispatcherURLs = append(dispatcherURLs, u)
	}

	parsedRpcURL, err := url.Parse(httpRpcURL)
	if err != nil {
		return nil, err
	}

	return &Pocket{
		Dispatchers: dispatcherURLs,
		RPCProvider: parsedRpcURL,
		client:      *httpClient.NewClient(),
	}, nil
}

func (p *Pocket) GetNetworkApplications(input GetNetworkApplicationsInput) ([]common.NetworkApplication, error) {
	options := struct {
		Opts GetNetworkApplicationsInput `json:"opts"`
	}{
		Opts: input,
	}

	res, err := p.perform(performRequestOptions{
		route: QueryApps,
		body:  options,
	})
	if err != nil {
		return nil, err
	}

	var applications *GetNetworkApplicationsOutput
	if err = json.NewDecoder(res.Body).Decode(&applications); err != nil {
		return nil, err
	}

	return applications.Result, nil
}

func (p *Pocket) getRandomDispatcher() string {
	return p.Dispatchers[rand.Intn(len(p.Dispatchers))].String()
}

func (p *Pocket) DispatchSession(options DispatchInput) (*Session, error) {
	res, err := p.perform(performRequestOptions{
		route: ClientDispatch,
		body:  options,
	})
	if err != nil {
		return nil, err
	}

	var dispatchOutput *DispatchOutput
	if err = json.NewDecoder(res.Body).Decode(&dispatchOutput); err != nil {
		return nil, err
	}

	return &dispatchOutput.Session, nil
}

func (p *Pocket) perform(options performRequestOptions) (*http.Response, error) {
	var finalRpcURL string
	if options.rpcURL != "" {
		finalRpcURL = options.rpcURL
	} else {
		if options.route == ClientDispatch {
			finalRpcURL = p.getRandomDispatcher()
		} else {
			finalRpcURL = p.RPCProvider.String()
		}
	}

	body, err := json.Marshal(options.body)
	if err != nil {
		return nil, err
	}

	url := finalRpcURL + string(options.route)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString(string(body)))
	if err != nil {
		return nil, err
	}
	res, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}
