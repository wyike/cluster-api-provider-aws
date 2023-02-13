/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gc

import (
	"context"
	"fmt"
	infrav1 "sigs.k8s.io/cluster-api-provider-aws/v2/api/v1beta2"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/elb"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

func (s *Service) deleteLoadBalancers(ctx context.Context, resources []*AWSResource) error {
	for _, resource := range resources {
		if !s.isELBResourceToDelete(resource, "loadbalancer") {
			s.scope.Info("Resource not a load balancer for deletion", "arn", resource.ARN.String())
			continue
		}
		fmt.Printf("found one lb: %s,%s,%s\n", resource.ARN.Service, resource.ARN.Resource, resource.ARN.Partition)
		switch {
		case strings.HasPrefix(resource.ARN.Resource, "loadbalancer/app/"):
			s.scope.Info("Deleting ALB for Service", "arn", resource.ARN.String())
			if err := s.deleteLoadBalancerV2(ctx, resource.ARN.String()); err != nil {
				return fmt.Errorf("deleting ALB: %w", err)
			}
		case strings.HasPrefix(resource.ARN.Resource, "loadbalancer/net/"):
			s.scope.Info("Deleting NLB for Service", "arn", resource.ARN.String())
			if err := s.deleteLoadBalancerV2(ctx, resource.ARN.String()); err != nil {
				return fmt.Errorf("deleting NLB: %w", err)
			}
		case strings.HasPrefix(resource.ARN.Resource, "loadbalancer/"):
			name := strings.ReplaceAll(resource.ARN.Resource, "loadbalancer/", "")
			s.scope.Info("Deleting classic ELB for Service", "arn", resource.ARN.String(), "name", name)
			if err := s.deleteLoadBalancer(ctx, name); err != nil {
				return fmt.Errorf("deleting classic ELB: %w", err)
			}
		default:
			s.scope.Trace("Unexpected elasticloadbalancing resource, ignoring", "arn", resource.ARN.String())
		}
	}

	s.scope.Info("Finished processing tagged resources for load balancers")

	return nil
}

func (s *Service) deleteTargetGroups(ctx context.Context, resources []*AWSResource) error {
	for _, resource := range resources {
		if !s.isELBResourceToDelete(resource, "targetgroup") {
			s.scope.Debug("Resource not a target group for deletion", "arn", resource.ARN.String())
			continue
		}
		fmt.Printf("found one sg: %s,%s,%s\n", resource.ARN.Service, resource.ARN.Resource, resource.ARN.Partition)
		name := strings.ReplaceAll(resource.ARN.Resource, "targetgroup/", "")
		if err := s.deleteTargetGroup(ctx, resource.ARN.String()); err != nil {
			return fmt.Errorf("deleting target group %s: %w", name, err)
		}
	}
	s.scope.Debug("Finished processing resources for target group deletion")

	return nil
}

func (s *Service) isELBResourceToDelete(resource *AWSResource, resourceName string) bool {
	if !s.isMatchingResource(resource, elb.ServiceName, resourceName) {
		return false
	}

	if serviceName := resource.Tags[serviceNameTag]; serviceName == "" {
		s.scope.Info("Resource wasn't created for a Service via CCM", "arn", resource.ARN.String(), "resource_name", resourceName)
		return false
	}

	return true
}

func (s *Service) deleteLoadBalancerV2(ctx context.Context, lbARN string) error {
	input := elbv2.DeleteLoadBalancerInput{
		LoadBalancerArn: aws.String(lbARN),
	}

	s.scope.Debug("Deleting v2 load balancer", "arn", lbARN)
	if _, err := s.elbv2Client.DeleteLoadBalancerWithContext(ctx, &input); err != nil {
		return fmt.Errorf("deleting v2 load balancer: %w", err)
	}

	return nil
}

func (s *Service) deleteLoadBalancer(ctx context.Context, name string) error {
	input := elb.DeleteLoadBalancerInput{
		LoadBalancerName: aws.String(name),
	}

	s.scope.Debug("Deleting classic load balancer", "name", name)
	if _, err := s.elbClient.DeleteLoadBalancerWithContext(ctx, &input); err != nil {
		return fmt.Errorf("deleting classic load balancer: %w", err)
	}

	return nil
}

func (s *Service) deleteTargetGroup(ctx context.Context, targetGroupARN string) error {
	input := elbv2.DeleteTargetGroupInput{
		TargetGroupArn: aws.String(targetGroupARN),
	}

	s.scope.Debug("Deleting target group", "arn", targetGroupARN)
	if _, err := s.elbv2Client.DeleteTargetGroupWithContext(ctx, &input); err != nil {
		return fmt.Errorf("deleting target group: %w", err)
	}

	return nil
}

func (s *Service) getAllNLBLoadBalancers(ctx context.Context, name string) error {
	var names []string
	err := s.elbv2Client.DescribeLoadBalancersPages(&elbv2.DescribeLoadBalancersInput{}, func(r *elbv2.DescribeLoadBalancersOutput, last bool) bool {
		for _, lb := range r.LoadBalancers {
			names = append(names, *lb.LoadBalancerArn)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
}

func (s *Service) getAllELBLoadBalancers(ctx context.Context, name string) error {
	var names []string
	err := s.ELBClient.DescribeLoadBalancersPages(&elb.DescribeLoadBalancersInput{}, func(r *elb.DescribeLoadBalancersOutput, last bool) bool {
		for _, lb := range r.LoadBalancerDescriptions {
			names = append(names, *lb.LoadBalancerName)
		}
		return true
	})
	if err != nil {
		return nil, err
	}
}

func (s *Service) getClusterOwnedELBLoadBalancers(ctx context.Context, name string) error {
	var ownedElbs []string
	lbChunks := chunkELBs(names)
	for _, chunk := range lbChunks {
		output, err := s.elbClient.DescribeTags(&elb.DescribeTagsInput{LoadBalancerNames: aws.StringSlice(chunk)})
		if err != nil {
			return nil, err
		}
		for _, tagDesc := range output.TagDescriptions {
			for _, tag := range tagDesc.Tags {
				if *tag.Key == tagKey && *tag.Value == string(infrav1.ResourceLifecycleOwned) {
					ownedElbs = append(ownedElbs, *tagDesc.LoadBalancerName)
				}
			}
		}
	}
}

func (s *Service) getClusterOwnedNLBLoadBalancers(ctx context.Context, name string) error {
	var ownedNlbs []string
	lbChunks := chunkELBs(names)
	for _, chunk := range lbChunks {
		output, err := s.ELBV2Client.DescribeTags(&elbv2.DescribeTagsInput{ResourceArns: aws.StringSlice(chunk)})
		if err != nil {
			return nil, err
		}
		for _, tagDesc := range output.TagDescriptions {
			for _, tag := range tagDesc.Tags {
				if *tag.Key == tagKey && *tag.Value == string(infrav1.ResourceLifecycleOwned) {
					ownedNlbs = append(ownedNlbs, *tagDesc.LoadBalancerName)
				}
			}
			for _, tag := range tagDesc.Tags {
				if *tag.Key == serviceNameTag {
					ownedNlbs = append(ownedNlbs, *tagDesc.LoadBalancerName)
				}
			}
		}
	}
}
func (s *Service) deleteLBServiceOwnedLoadBalancers() error {
	s.getAllELBLoadBalancers()
	s.getClusterOwnedELBLoadBalancers()

	s.getAllNLBLoadBalancers()
	s.getClusterOwnedELBLoadBalancers()

}

func (s *Service) describeTargetgroups() ([]infrav1.SecurityGroup, error) {
	groups, err := s.ELBV2Client.DescribeTargetGroups(&elbv2.DescribeTargetGroupsInput{
		LoadBalancerArn: aws.String(arn),
	})
	if err != nil {
		return fmt.Errorf("failed to gather target groups for LB: %w", err)
	}
}
