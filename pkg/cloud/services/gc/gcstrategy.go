package gc

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi/resourcegroupstaggingapiiface"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	rgapi "github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-aws/v2/pkg/cloud"
)

type gcStrategy interface {
	Cleanup(ctx context.Context) error
}

func newDefaultGcStrategy(clusterScope cloud.ClusterScoper, taggingClient resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI, cleanup ResourceCleanupFuncs) *defaultGcStrategy {
	gs := &defaultGcStrategy{
		cleanups:              cleanup,
		resourceTaggingClient: taggingClient,
		scope:                 clusterScope,
	}

	return gs
}

func newSecondaryGcStrategy(collect ResourceCollectFuncs, cleanup ResourceCleanupFuncs) *secondaryGcStrategy {
	gs := &secondaryGcStrategy{
		collects: collect,
		cleanups: cleanup,
	}

	return gs
}

// defaultSubnetPlacementStrategy is the default strategy for subnet placement.
type defaultGcStrategy struct {
	scope                 cloud.ClusterScoper
	cleanups              ResourceCleanupFuncs
	resourceTaggingClient resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI
}

// defaultSubnetPlacementStrategy is the default strategy for subnet placement.
type secondaryGcStrategy struct {
	collects ResourceCollectFuncs
	cleanups ResourceCleanupFuncs
}

func (s *defaultGcStrategy) Cleanup(ctx context.Context) error {
	serviceTag := infrav1.ClusterAWSCloudProviderTagKey(s.scope.KubernetesClusterName())

	awsInput := rgapi.GetResourcesInput{
		ResourceTypeFilters: nil,
		TagFilters: []*rgapi.TagFilter{
			{
				Key:    aws.String(serviceTag),
				Values: []*string{aws.String(string(infrav1.ResourceLifecycleOwned))},
			},
		},
	}

	awsOutput, err := s.resourceTaggingClient.GetResourcesWithContext(ctx, &awsInput)
	if err != nil {
		return err
	}

	resources := []*AWSResource{}

	for i := range awsOutput.ResourceTagMappingList {
		mapping := awsOutput.ResourceTagMappingList[i]
		parsedArn, err := arn.Parse(*mapping.ResourceARN)
		if err != nil {
			return fmt.Errorf("parsing resource arn %s: %w", *mapping.ResourceARN, err)
		}

		tags := map[string]string{}
		for _, rgTag := range mapping.Tags {
			tags[*rgTag.Key] = *rgTag.Value
		}

		resources = append(resources, &AWSResource{
			ARN:  &parsedArn,
			Tags: tags,
		})
	}

	if deleteErr := s.cleanups.Execute(ctx, resources); deleteErr != nil {
		return fmt.Errorf("deleting resources: %w", deleteErr)
	}

	return nil
}

func (s *secondaryGcStrategy) Cleanup(ctx context.Context) error {
	resources, err := s.collects.Execute()
	if err != nil {
		return err
	}

	if deleteErr := s.cleanups.Execute(ctx, resources); deleteErr != nil {
		return fmt.Errorf("deleting resources: %w", deleteErr)
	}

	return nil
}
