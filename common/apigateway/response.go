package apigateway

import (
	"encoding/json"

	"github.com/Pocket/global-services/lib/logger"
	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"
)

// ErrorResponse represents an API error
type ErrorResponse struct {
	HTTPCode int    `json:"http_code"`
	Message  string `json:"message"`
}

// NewErrorResponse returns an error response
func NewErrorResponse(statusCode int, err error) *events.APIGatewayProxyResponse {
	return NewJSONResponse(statusCode, ErrorResponse{
		HTTPCode: statusCode,
		Message:  err.Error(),
	})
}

// NewJSONResponse creates a new JSON response given a serializable val
func NewJSONResponse(statusCode int, val interface{}) *events.APIGatewayProxyResponse {
	data, _ := json.Marshal(val)
	return &events.APIGatewayProxyResponse{
		StatusCode:      statusCode,
		IsBase64Encoded: false,
		Body:            string(data),
		Headers: map[string]string{
			"Content-Type":                "application/json",
			"Access-Control-Allow-Origin": "*",
		},
	}
}

// LogAndReturnError logs the given error to the console and terminates
// the response with a status 500 showing the error
func LogAndReturnError(err error) *events.APIGatewayProxyResponse {
	logger.Log.WithFields(logrus.Fields{
		"error": err.Error(),
	}).Error(err)

	return NewErrorResponse(500, err)
}
