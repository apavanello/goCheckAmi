package main

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

func main() {
	ctx := context.TODO()

	// Custom resolver for LocalStack
	customResolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           "http://localhost:4566",
			SigningRegion: "us-east-1",
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(customResolver),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		log.Fatalf("unable to load SDK config: %v", err)
	}

	// 1. Setup SSM
	ssmClient := ssm.NewFromConfig(cfg)
	params := map[string]string{
		"/app/prod/db_url":  "jdbc:mysql://prod-db:3306/db",
		"/app/prod/api_key": "secret-key-prod",
		"/app/dev/db_url":   "jdbc:mysql://dev-db:3306/db",
		"service-a-config":  "some-config",
	}

	fmt.Println("Populating SSM Parameters...")
	for name, val := range params {
		_, err := ssmClient.PutParameter(ctx, &ssm.PutParameterInput{
			Name:      aws.String(name),
			Value:     aws.String(val),
			Type:      ssmtypes.ParameterTypeString,
			Overwrite: aws.Bool(true),
		})
		if err != nil {
			log.Printf("Failed to put parameter %s: %v", name, err)
		} else {
			fmt.Printf("Put parameter %s\n", name)
		}
	}

	// 2. Setup EC2
	ec2Client := ec2.NewFromConfig(cfg)
	fmt.Println("Launching EC2 Instances...")

	instances := []struct {
		Name string
		AMI  string
	}{
		{"WebServer-Prod", "ami-12345678"},
		{"Worker-Dev", "ami-87654321"},
	}

	for _, inst := range instances {
		_, err := ec2Client.RunInstances(ctx, &ec2.RunInstancesInput{
			ImageId:      aws.String(inst.AMI),
			InstanceType: types.InstanceTypeT2Micro,
			MinCount:     aws.Int32(1),
			MaxCount:     aws.Int32(1),
			TagSpecifications: []types.TagSpecification{
				{
					ResourceType: types.ResourceTypeInstance,
					Tags: []types.Tag{
						{Key: aws.String("Name"), Value: aws.String(inst.Name)},
					},
				},
			},
		})
		if err != nil {
			log.Printf("Failed to run instance %s: %v", inst.Name, err)
		} else {
			fmt.Printf("Launched instance %s\n", inst.Name)
		}
	}

	fmt.Println("Done populating LocalStack.")
}
