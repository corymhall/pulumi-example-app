package main

import (
	"fmt"

	ec2_classic "github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ec2"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/ecs"
	"github.com/pulumi/pulumi-aws/sdk/v6/go/aws/servicediscovery"
	"github.com/pulumi/pulumi-awsx/sdk/v2/go/awsx/ec2"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type Network struct {
	vpc       *ec2.Vpc
	cluster   *ecs.Cluster
	namespace *servicediscovery.PrivateDnsNamespace
}

func NewNetwork(ctx *pulumi.Context) (*Network, error) {
	var err error
	network := &Network{}

	as := ec2.SubnetAllocationStrategyAuto
	network.vpc, err = ec2.NewVpc(ctx, "vpc", &ec2.VpcArgs{
		NatGateways:    &ec2.NatGatewayConfigurationArgs{Strategy: ec2.NatGatewayStrategySingle},
		SubnetStrategy: &as,
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating vpc: %w", err)
	}

	namespace, err := servicediscovery.NewPrivateDnsNamespace(ctx, "chall.dev", &servicediscovery.PrivateDnsNamespaceArgs{
		Vpc: network.vpc.VpcId,
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating namespace: %w", err)
	}
	network.namespace = namespace
	network.cluster, err = ecs.NewCluster(ctx, "cluster", &ecs.ClusterArgs{
		ServiceConnectDefaults: &ecs.ClusterServiceConnectDefaultsArgs{
			Namespace: namespace.Arn,
		},
		Settings: ecs.ClusterSettingArray{
			ecs.ClusterSettingArgs{
				Name:  pulumi.String("containerInsights"),
				Value: pulumi.String("enabled"),
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("Error creating cluster: %w", err)
	}

	return network, nil
}

func egressAll() ec2_classic.SecurityGroupEgressArray {
	return ec2_classic.SecurityGroupEgressArray{
		ec2_classic.SecurityGroupEgressArgs{
			CidrBlocks:  pulumi.ToStringArray([]string{"0.0.0.0/0"}),
			Description: pulumi.String("Egress all"),
			Protocol:    pulumi.String("-1"),
			FromPort:    pulumi.Int(0),
			ToPort:      pulumi.Int(0),
		},
	}
}

func ingress(port int, sg ...*ec2_classic.SecurityGroup) ec2_classic.SecurityGroupIngressArray {
	sgs := pulumi.StringArray{}
	for i := range sg {
		sgs = append(sgs, sg[i].ID())
	}
	return ec2_classic.SecurityGroupIngressArray{
		ec2_classic.SecurityGroupIngressArgs{
			FromPort:       pulumi.Int(port),
			ToPort:         pulumi.Int(port),
			Protocol:       pulumi.String("tcp"),
			SecurityGroups: sgs,
		},
	}
}
