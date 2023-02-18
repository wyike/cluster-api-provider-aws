package gc

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

const (
	fakePartition     = "aws"
	fakeRegion        = "fake-region"
	fakeAccount       = "fake-account"
	elbService        = "elasticloadbalancing"
	elbResourcePrefix = "loadbalancer/"
	sgService         = "ec2"
	sgResourcePrefix  = "security-group/"

	// maxDescribeTagsRequest is the maximum number of resources for the DescribeTags API call
	// see: https://docs.aws.amazon.com/elasticloadbalancing/latest/APIReference/API_DescribeTags.html.
	maxDescribeTagsRequest = 20
)

// composeArn composes a resource arn with correct service and resource, but fake partition, region and account.
func composeArn(service, resource string) string {
	return "arn:" + fakePartition + ":" + service + ":" + fakeRegion + ":" + fakeAccount + ":" + resource
}

// composeAWSResource composes *AWSResource object for an aws resource.
func composeAWSResource(resourceARN *string, resourceTags interface{}) (*AWSResource, error) {
	parsedArn, err := arn.Parse(*resourceARN)
	if err != nil {
		return nil, fmt.Errorf("parsing resource arn %s: %w", *resourceARN, err)
	}

	tags := map[string]string{}
	switch v := resourceTags.(type) {
	case []*elb.Tag:
		for _, tag := range v {
			tags[*tag.Key] = *tag.Value
		}
	case []*elbv2.Tag:
		for _, tag := range v {
			tags[*tag.Key] = *tag.Value
		}
	case []*ec2.Tag:
		for _, tag := range v {
			tags[*tag.Key] = *tag.Value
		}
	default:
		return nil, fmt.Errorf("unknown tag type")
	}

	resource := &AWSResource{
		ARN:  &parsedArn,
		Tags: tags,
	}

	return resource, nil
}

// chunkResources is similar to https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/main/pkg/cloud/services/elb/loadbalancer.go#L1488.
func chunkResources(names []string) [][]string {
	var chunked [][]string
	for i := 0; i < len(names); i += maxDescribeTagsRequest {
		end := i + maxDescribeTagsRequest
		if end > len(names) {
			end = len(names)
		}
		chunked = append(chunked, names[i:end])
	}
	return chunked
}
