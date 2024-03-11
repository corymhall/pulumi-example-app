package main

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, event events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	slog.Info("Request received", slog.Any("event", event))
	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Body:       "Hello, World!",
	}, nil
}

func main() {
	lambda.Start(handler)
}
