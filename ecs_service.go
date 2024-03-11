package main

import (
	"fmt"

	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/cloudwatch"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecs"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/servicediscovery"
	"github.com/pulumi/pulumi-docker/sdk/v4/go/docker"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type EcsServiceArgs struct {
	image   *docker.Image
	network *Network
	api     *Api
}

type EcsService struct {
	cloudmapService *servicediscovery.Service
	port            int
	sg              *ec2.SecurityGroup
}

func NewEcsService(ctx *pulumi.Context, args EcsServiceArgs) (*EcsService, error) {
	ecsService := &EcsService{
		port: 3000,
	}
	region, _ := aws.GetRegion(ctx, nil)
	logGroup, _ := cloudwatch.NewLogGroup(ctx, "app-log-group", &cloudwatch.LogGroupArgs{
		RetentionInDays: pulumi.IntPtr(1),
	})
	containerDef := pulumi.JSONMarshal([]interface{}{
		map[string]interface{}{
			"name":  "app",
			"image": args.image.RepoDigest,
			"portMappings": []map[string]interface{}{
				{
					"containerPort": 3000,
				},
			},
			"logConfiguration": map[string]interface{}{
				"logDriver": "awslogs",
				"options": map[string]interface{}{
					"awslogs-group":         logGroup.Name,
					"awslogs-region":        region.Name,
					"awslogs-stream-prefix": "app",
				},
			},
		},
	})

	execAssumeRolePolicy, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
		Statements: []iam.GetPolicyDocumentStatement{
			{
				Actions: []string{"sts:AssumeRole"},
				Principals: []iam.GetPolicyDocumentStatementPrincipal{
					{Type: "Service", Identifiers: []string{"ecs-tasks.amazonaws.com"}},
				},
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("Error creating execAssumeRolePolicy: %w", err)
	}
	executionRole, err := iam.NewRole(ctx, "execution-role", &iam.RoleArgs{
		AssumeRolePolicy:  pulumi.String(execAssumeRolePolicy.Json),
		ManagedPolicyArns: pulumi.ToStringArray([]string{string(iam.ManagedPolicyAmazonECSTaskExecutionRolePolicy)}),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating execution role: %w", err)
	}
	taskAssumeRolePolicy, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
		Statements: []iam.GetPolicyDocumentStatement{
			{
				Actions: []string{"sts:AssumeRole"},
				Principals: []iam.GetPolicyDocumentStatementPrincipal{
					{Type: "Service", Identifiers: []string{"ecs-tasks.amazonaws.com"}},
				},
				// TODO: how to get the account id
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating taskAssumeRolePolicy: %w", err)
	}
	// taskRolePolicy, err := iam.GetPolicyDocument(ctx, &iam.GetPolicyDocumentArgs{
	// 	Statements: []iam.GetPolicyDocumentStatement{
	// 		{
	// 			Actions:   []string{},
	// 			Resources: []string{},
	// 		},
	// 	},
	// })
	// if err != nil {
	// 	return nil, fmt.Errorf("Error creating taskRolePolicy: %w", err)
	// }
	taskRole, err := iam.NewRole(ctx, "task-role", &iam.RoleArgs{
		AssumeRolePolicy: pulumi.String(taskAssumeRolePolicy.Json),
		// InlinePolicies: iam.RoleInlinePolicyArray{
		// 	iam.RoleInlinePolicyArgs{
		// 		Name:   pulumi.String("default-policy"),
		// 		Policy: pulumi.String(taskRolePolicy.Json),
		// 	},
		// },
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating execution role: %w", err)
	}
	taskdef, err := ecs.NewTaskDefinition(ctx, "taskdef", &ecs.TaskDefinitionArgs{
		ContainerDefinitions:    containerDef,
		Family:                  pulumi.String("app"),
		Cpu:                     pulumi.String("256"),
		ExecutionRoleArn:        executionRole.Arn,
		Memory:                  pulumi.String("512"),
		TaskRoleArn:             taskRole.Arn,
		RequiresCompatibilities: pulumi.ToStringArray([]string{"FARGATE"}),
		NetworkMode:             pulumi.String("awsvpc"),
		RuntimePlatform: ecs.TaskDefinitionRuntimePlatformArgs{
			CpuArchitecture:       pulumi.String("ARM64"),
			OperatingSystemFamily: pulumi.String("LINUX"),
		},
	}, pulumi.DependsOn([]pulumi.Resource{args.image}))
	if err != nil {
		return nil, fmt.Errorf("Error creating taskdef: %w", err)
	}
	sg, err := ec2.NewSecurityGroup(ctx, "service-sg", &ec2.SecurityGroupArgs{
		Egress:              egressAll(),
		VpcId:               args.network.vpc.VpcId,
		Ingress:             ingress(3000, args.api.sg),
		RevokeRulesOnDelete: pulumi.BoolPtr(true),
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating security group: %w", err)
	}
	ecsService.sg = sg

	sd, err := servicediscovery.NewService(ctx, "cloudmap-service", &servicediscovery.ServiceArgs{
		NamespaceId: args.network.namespace.ID(),
		DnsConfig: &servicediscovery.ServiceDnsConfigArgs{
			NamespaceId:   args.network.namespace.ID(),
			RoutingPolicy: pulumi.String("MULTIVALUE"),
			DnsRecords: servicediscovery.ServiceDnsConfigDnsRecordArray{
				servicediscovery.ServiceDnsConfigDnsRecordArgs{
					Ttl:  pulumi.Int(300),
					Type: pulumi.String("SRV"),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating servicediscovery service: %w", err)
	}
	ecsService.cloudmapService = sd
	args.api.registerCloudmapService(ctx, ecsService.cloudmapService)

	ecs.NewService(ctx, "service", &ecs.ServiceArgs{
		Cluster:                  args.network.cluster.Arn,
		DesiredCount:             pulumi.IntPtr(1),
		DeploymentMaximumPercent: pulumi.IntPtr(200),
		ServiceRegistries: &ecs.ServiceServiceRegistriesArgs{
			ContainerName: pulumi.String("app"),
			ContainerPort: pulumi.IntPtr(3000),
			RegistryArn:   sd.Arn,
		},
		DeploymentMinimumHealthyPercent: pulumi.IntPtr(100),
		DeploymentCircuitBreaker: ecs.ServiceDeploymentCircuitBreakerArgs{
			Enable:   pulumi.Bool(true),
			Rollback: pulumi.Bool(true),
		},
		LaunchType:         pulumi.String("FARGATE"),
		WaitForSteadyState: pulumi.BoolPtr(true),
		NetworkConfiguration: ecs.ServiceNetworkConfigurationArgs{
			AssignPublicIp: pulumi.BoolPtr(false),
			SecurityGroups: pulumi.StringArray{ecsService.sg.ID()},
			Subnets:        args.network.vpc.PrivateSubnetIds,
		},
		TaskDefinition: taskdef.Arn,
	})

	return ecsService, nil
}
