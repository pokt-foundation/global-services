package pocket

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand"
	"net/http"
	"net/url"

	httpClient "github.com/Pocket/global-dispatcher/lib/http"
)

// PocketJsonRpcClient is a struct containing rpc calls to the PocketJsonRpcClient's blockchain network
type PocketJsonRpcClient struct {
	RPCProvider *url.URL
	Dispatchers []*url.URL
	client      httpClient.Client
}

type performRequestOptions struct {
	route  V1RPCRoutes
	rpcURL string
	body   interface{}
}

func NewPocketClient(httpRpcURL string, dispatchers []string) (*PocketJsonRpcClient, error) {
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

	return &PocketJsonRpcClient{
		Dispatchers: dispatcherURLs,
		RPCProvider: parsedRpcURL,
		client:      *httpClient.NewClient(),
	}, nil
}

func (p *PocketJsonRpcClient) GetBlockHeight() (int, error) {
	res, err := p.perform(performRequestOptions{
		route: Height,
	})
	if err != nil {
		return 0, err
	}

	var height struct {
		Height int `json:"height"`
	}

	return height.Height, json.NewDecoder(res.Body).Decode(&height)
}

func (p *PocketJsonRpcClient) GetNetworkApplications(input GetNetworkApplicationsInput) ([]NetworkApplication, error) {
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

func (p *PocketJsonRpcClient) getRandomDispatcher() string {
	return p.Dispatchers[rand.Intn(len(p.Dispatchers))].String()
}

func (p *PocketJsonRpcClient) DispatchSession(options DispatchInput) (*Session, error) {
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

	// Consumers only expect to have the session struct on the cache data, so the
	// dispatcher blockheight is embedded within the session struct for convenience,
	dispatchOutput.Session.BlockHeight = dispatchOutput.BlockHeight

	return &dispatchOutput.Session, nil
}

func (p *PocketJsonRpcClient) perform(options performRequestOptions) (*http.Response, error) {
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
