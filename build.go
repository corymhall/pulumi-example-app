package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecr"
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type EcrImage struct {
	image *docker.Image
}

func NewEcrDockerBuild(ctx *pulumi.Context) (*EcrImage, error) {
	ecrImage := &EcrImage{}
	repo, err := ecr.NewRepository(ctx, "registry", &ecr.RepositoryArgs{
		ForceDelete: pulumi.BoolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating repo: %w", err)
	}
	authToken := ecr.GetAuthorizationTokenOutput(ctx, ecr.GetAuthorizationTokenOutputArgs{
		RegistryId: repo.RegistryId,
	})
	ecrImage.image, err = docker.NewImage(ctx, "app-image", &docker.ImageArgs{
		Registry: docker.RegistryArgs{
			Username: authToken.UserName(),
			Password: pulumi.ToSecret(authToken.ApplyT(func(authToken ecr.GetAuthorizationTokenResult) (*string, error) {
				return &authToken.Password, nil
			})).(pulumi.StringPtrOutput),
		},
		Build: docker.DockerBuildArgs{
			Platform:   pulumi.String("linux/arm64"),
			Context:    pulumi.String("."),
			Dockerfile: pulumi.String("app/Dockerfile"),
		},
		ImageName: repo.RepositoryUrl.ApplyT(func(url string) string {
			return fmt.Sprintf("%s:latest", url)
		}).(pulumi.StringOutput),
	})

	return ecrImage, nil
}
