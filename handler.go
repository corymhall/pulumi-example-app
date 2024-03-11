package main

import (
	"fmt"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi-command/sdk/go/command/local"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type LambdaHandler struct {
}

type LambdaHandlerArgs struct {
	api *Api
}

func NewLambdaHandler(ctx *pulumi.Context, args LambdaHandlerArgs) (*LambdaHandler, error) {
	lh := &LambdaHandler{}

	_, err := local.Run(ctx, &local.RunArgs{
		Dir: pulumi.StringRef("."),
		Command: strings.Join([]string{
			"rm -rf asset && mkdir asset",
			"GOOS=linux GOARCH=arm64 go build -mod=readonly -o ./asset/bootstrap ./cmd/app",
			"chmod +x ./asset/bootstrap",
		}, " && "),
		AssetPaths: []string{"asset/bootstrap"},
	})
	if err != nil {
		return nil, fmt.Errorf("Error running local command: %w", err)
	}

	assumeRolePolicy, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
		Statements: []iam.GetPolicyDocumentStatement{
			{
				Actions: []string{"sts:AssumeRole"},
				Principals: []iam.GetPolicyDocumentStatementPrincipal{
					{Type: "Service", Identifiers: []string{"lambda.amazonaws.com"}},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating AssumeRolePolicy: %w", err)
	}
	executionRole, err := iam.NewRole(ctx, "lambda-execution-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(assumeRolePolicy.Json),
		ManagedPolicyArns: pulumi.ToStringArray([]string{
			string(iam.ManagedPolicyAWSLambdaBasicExecutionRole),
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating execution role: %w", err)
	}

	code := pulumi.NewAssetArchive(map[string]interface{}{"bootstrap": pulumi.NewFileAsset("./asset/bootstrap")})
	handler, err := lambda.NewFunction(ctx, "handler", &lambda.FunctionArgs{
		Architectures: pulumi.ToStringArray([]string{"arm64"}),
		Role:          executionRole.Arn,
		Code:          code,
		Handler:       pulumi.String("bootstrap"),
		Runtime:       pulumi.String("provided.al2023"),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating lambda function: %w", err)
	}

	args.api.registerLambda(ctx, handler)

	return lh, nil
}
