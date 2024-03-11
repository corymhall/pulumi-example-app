package main

import (
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		network, err := NewNetwork(ctx)
		if err != nil {
			return err
		}

		build, err := NewEcrDockerBuild(ctx)
		if err != nil {
			return err
		}

		api, err := NewApi(ctx, ApiArgs{
			network: network,
		})
		if err != nil {
			return err
		}

		_, err = NewLambdaHandler(ctx, LambdaHandlerArgs{
			api: api,
		})
		if err != nil {
			return err
		}

		_, err = NewEcsService(ctx, EcsServiceArgs{
			image:   build.image,
			api:     api,
			network: network,
		})
		if err != nil {
			return err
		}

		return nil
	})
}
