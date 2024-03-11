package main

import (
	"fmt"

	apigwv2 "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/apigatewayv2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/servicediscovery"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type ApiArgs struct {
	network *Network
}

type Api struct {
	api          *apigwv2.Api
	link         *apigwv2.VpcLink
	defaultStage *apigwv2.Stage
	sg           *ec2.SecurityGroup
}

func NewApi(ctx *pulumi.Context, args ApiArgs) (*Api, error) {
	api := &Api{}
	var err error
	api.api, err = apigwv2.NewApi(ctx, "api", &apigwv2.ApiArgs{
		ProtocolType: pulumi.String("HTTP"),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating api: %w", err)
	}

	stage, _ := apigwv2.NewStage(ctx, "default-stage", &apigwv2.StageArgs{
		ApiId:      api.api.ID(),
		AutoDeploy: pulumi.Bool(true),
	})
	api.defaultStage = stage

	sg, err := ec2.NewSecurityGroup(ctx, "vpc-link-sg", &ec2.SecurityGroupArgs{
		VpcId:               args.network.vpc.VpcId,
		Egress:              egressAll(),
		RevokeRulesOnDelete: pulumi.Bool(true),
	})
	api.sg = sg
	api.link, err = apigwv2.NewVpcLink(ctx, "vpc-link", &apigwv2.VpcLinkArgs{
		SecurityGroupIds: pulumi.StringArray{sg.ID()},
		SubnetIds:        args.network.vpc.PrivateSubnetIds,
	})

	if err != nil {
		return nil, fmt.Errorf("Error creating sg: %w", err)
	}

	ctx.Export("url", stage.InvokeUrl)

	return api, nil
}

func (a *Api) registerCloudmapService(ctx *pulumi.Context, service *servicediscovery.Service) error {
	integration, err := apigwv2.NewIntegration(ctx, "integration", &apigwv2.IntegrationArgs{
		ApiId:             a.api.ID(),
		ConnectionId:      a.link.ID(),
		ConnectionType:    pulumi.String("VPC_LINK"),
		IntegrationMethod: pulumi.String("ANY"),
		IntegrationType:   pulumi.String("HTTP_PROXY"),
		IntegrationUri:    service.Arn,
	})
	if err != nil {
		return fmt.Errorf("Error creating integration: %w", err)
	}

	_, err = apigwv2.NewRoute(ctx, "route", &apigwv2.RouteArgs{
		ApiId:    a.api.ID(),
		RouteKey: pulumi.String("GET /pets"),
		Target:   pulumi.Sprintf("integrations/%s", integration.ID()),
	})
	if err != nil {
		return fmt.Errorf("Error creating route: %w", err)
	}

	return nil
}

func (a *Api) registerLambda(ctx *pulumi.Context, handler *lambda.Function) error {
	integration, err := apigwv2.NewIntegration(ctx, "lambda-integration", &apigwv2.IntegrationArgs{
		ApiId:             a.api.ID(),
		IntegrationMethod: pulumi.String("GET"),
		IntegrationType:   pulumi.String("AWS_PROXY"),
		IntegrationUri:    handler.Arn,
	})
	if err != nil {
		return fmt.Errorf("Error creating integration: %w", err)
	}

	_, err = apigwv2.NewRoute(ctx, "lambda-route", &apigwv2.RouteArgs{
		ApiId:    a.api.ID(),
		RouteKey: pulumi.String("GET /"),
		Target:   pulumi.Sprintf("integrations/%s", integration.ID()),
	})
	if err != nil {
		return fmt.Errorf("Error creating route: %w", err)
	}

	lambda.NewPermission(ctx, "apigw-lambda-permission", &lambda.PermissionArgs{
		Action:    pulumi.String("lambda:InvokeFunction"),
		SourceArn: pulumi.Sprintf("%s/*/*/", a.api.ExecutionArn),
		// SourceArn: pulumi.Sprintf("%s/%s/GET/", a.api.ExecutionArn, a.defaultStage.Name),
		Function:  handler.Name,
		Principal: pulumi.String("apigateway.amazonaws.com"),
	})

	return nil
}
