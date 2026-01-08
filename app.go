package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"gopkg.in/ini.v1"
)

// App struct
type App struct {
	ctx context.Context
}

type EC2Instance struct {
	Name string `json:"name"`
	AMI  string `json:"ami"`
}

type AWSResult struct {
	Parameters []string      `json:"parameters"`
	Instances  []EC2Instance `json:"instances"`
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// getEndpointFromConfig tries to read 'endpoint_url' from ~/.aws/config for a profile
func (a *App) getEndpointFromConfig(profile string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	cfgPath := filepath.Join(home, ".aws", "config")
	cfg, err := ini.Load(cfgPath)
	if err != nil {
		return ""
	}

	// Try standard profile name format
	sectionName := "profile " + profile
	if profile == "default" {
		sectionName = "default"
	}

	section := cfg.Section(sectionName)
	if !section.HasKey("endpoint_url") {
		// Fallback: maybe user didn't use "profile " prefix for some reason or it's just "localstack"
		// But AWS config standard is [profile name] except for default.
		// Let's try just the name if headers didn't match
		section = cfg.Section(profile)
	}

	if section.HasKey("endpoint_url") {
		return section.Key("endpoint_url").String()
	}
	return ""
}

// ListProfiles reads the AWS config file and returns a list of available profiles
func (a *App) ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home: %w", err)
	}

	configPath := filepath.Join(home, ".aws", "config")
	// If config doesn't exist, try credentials
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = filepath.Join(home, ".aws", "credentials")
	}

	// If neither exist, return empty
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return []string{}, nil
	}

	cfg, err := ini.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load aws config: %w", err)
	}

	var profiles []string
	for _, section := range cfg.Sections() {
		name := section.Name()
		if name == "DEFAULT" {
			continue // skip global default section from ini package if present
		}

		// AWS config profiles are often named "profile name", except "default"
		if strings.HasPrefix(name, "profile ") {
			profiles = append(profiles, strings.TrimPrefix(name, "profile "))
		} else {
			// In credentials file or if it's just "default"
			profiles = append(profiles, name)
		}
	}
	return profiles, nil
}

// Processing handles the main logic: Auth, SSM, EC2
func (a *App) Processing(profile string, filter string) (*AWSResult, error) {
	// 0. Check for custom endpoint (LocalStack support)
	endpointURL := a.getEndpointFromConfig(profile)

	loadOpts := []func(*config.LoadOptions) error{
		config.WithSharedConfigProfile(profile),
	}

	if endpointURL != "" {
		resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:           endpointURL,
				SigningRegion: region, // Use region from config or default
			}, nil
		})
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(resolver))

		// 0.1 Inject dummy credentials for LocalStack to prevent SDK from falling back to EC2 IMDS
		// and failing with network errors (LocalStack accepts any non-empty creds).
		loadOpts = append(loadOpts, config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     "test",
				SecretAccessKey: "test",
				SessionToken:    "test",
				Source:          "HardcodedLocalStackCredentials",
			}, nil
		})))
	}

	// 1. Load AWS Config
	cfg, err := config.LoadDefaultConfig(a.ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	// 2. Validate Auth (check identity)
	stsClient := sts.NewFromConfig(cfg)
	_, err = stsClient.GetCallerIdentity(a.ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		// If we are using a custom endpoint (e.g. LocalStack), do not attempt SSO login.
		// Use the error from STS as the source of truth.
		if endpointURL != "" {
			return nil, fmt.Errorf("failed to validate identity with custom endpoint %q: %w. Ensure LocalStack is running and credentials are configured", endpointURL, err)
		}

		log.Printf("Token invalid or expired. Attempting SSO login for profile: %s", profile)

		// Check if 'aws' is in PATH before trying to run it
		if _, pathErr := exec.LookPath("aws"); pathErr != nil {
			return nil, fmt.Errorf("aws cli not found in PATH, cannot perform sso login: %w", err)
		}

		// Run aws sso login
		cmd := exec.Command("aws", "sso", "login", "--profile", profile)
		// cmd.Stdout = os.Stdout // If we want to see output in console
		// cmd.Stderr = os.Stderr
		// This might open a browser window and wait.
		if runErr := cmd.Run(); runErr != nil {
			return nil, fmt.Errorf("aws sso login failed: %w", runErr)
		}

		// Reload config after login
		cfg, err = config.LoadDefaultConfig(a.ctx, loadOpts...)
		if err != nil {
			return nil, fmt.Errorf("unable to reload SDK config after login: %v", err)
		}
	}

	result := &AWSResult{}

	// 3. SSM Parameters
	ssmClient := ssm.NewFromConfig(cfg)
	// Strategy: If filter ends with *, treat as path? Or just describe params?
	// User said: "considere um wildcard no fim do filtro mas nao no inicio" -> prefix match.
	// If it looks like a path (starts with /), use path?
	// Actually DescribeParameters filters are limited. "Name" filter supports "BeginsWith".
	// Let's use DescribeParameters with filter "Name" BeginsWith input (trimmed of *)

	cleanFilter := strings.TrimSuffix(filter, "*")
	// If filter is empty, maybe fetch all? Let's assume user wants to filter something.

	var params []string
	// Note: Verify if "BeginsWith" is default or explicit?
	// The AWS SDK 'ParametersFilter' behavior depends on usage.
	// For DescribeParameters, 'Name' filter automatically does exact match.
	// Wait, DescribeParameters supports "Name" with "BeginsWith" ONLY for GetParametersByPath?
	// No, DescribeParameters has 'Filters' (Key, Values). Keys: Name, Type, KeyId.
	// Documentation says: "ParametersFilterKeyName ... The name of the parameter. ... The results include parameters that match the specified name. If you use the wildcard character (*), the results include parameters that match the specified name pattern."
	// So if user passed "foo*", we can just pass that directly?
	// User said "wildcard at end but not start".
	// I will just pass the filter as is if it has *, or append * if logic requires.
	// But user said "considere um wildcard no fim do filtro mas nao no inicio".
	// It means if user types "prod", I should search for "prod*".
	// If I pass "prod*" to filter values, it should work.

	searchFilter := cleanFilter
	if !strings.HasSuffix(searchFilter, "*") {
		// Just to be safe, AWS might need explicit * for Contains/BeginsWith behavior in DescribeParameters?
		// Actually for DescribeParameters: "Allowed values: Name, Type, KeyId".
		// And for values: "You can use the wildcard character (*)."
		// So if user input is "foo", and I want "foo*", I should append *.
		// But if user input `foo*` already, I leave it.
	}
	// Let's ensure there is one * at end.
	if !strings.HasSuffix(searchFilter, "*") {
		searchFilter = searchFilter + "*"
	}

	paginator := ssm.NewDescribeParametersPaginator(ssmClient, &ssm.DescribeParametersInput{
		Filters: []ssmtypes.ParametersFilter{
			{
				Key:    ssmtypes.ParametersFilterKeyName,
				Values: []string{searchFilter},
			},
		},
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(a.ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list params: %w", err)
		}
		for _, p := range page.Parameters {
			if p.Name != nil {
				params = append(params, *p.Name)
			}
		}
	}
	result.Parameters = params

	// 4. EC2 Instances
	ec2Client := ec2.NewFromConfig(cfg)
	ec2Pager := ec2.NewDescribeInstancesPaginator(ec2Client, &ec2.DescribeInstancesInput{})

	var instances []EC2Instance
	for ec2Pager.HasMorePages() {
		page, err := ec2Pager.NextPage(a.ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}
		for _, res := range page.Reservations {
			for _, inst := range res.Instances {
				var name string
				for _, tag := range inst.Tags {
					if *tag.Key == "Name" {
						name = *tag.Value
						break
					}
				}
				ami := ""
				if inst.ImageId != nil {
					ami = *inst.ImageId
				}
				instances = append(instances, EC2Instance{
					Name: name,
					AMI:  ami,
				})
			}
		}
	}
	result.Instances = instances

	return result, nil
}
