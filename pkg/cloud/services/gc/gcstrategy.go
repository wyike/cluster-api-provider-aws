package gc

type collector interface {
	addCollectFuncs(s *Service) ResourceCollectFuncs
}

type defaultCollector ResourceCollectFuncs
type secondaryCollector ResourceCollectFuncs

func (c *defaultCollector) addCollectFuncs(s *Service) ResourceCollectFuncs {
	return []ResourceCollectFunc{
		s.defaultGetResources,
	}
}

func (c *secondaryCollector) addCollectFuncs(s *Service) ResourceCollectFuncs {
	return []ResourceCollectFunc{
		s.getProviderOwnedLoadBalancers,
		s.getProviderOwnedLoadBalancersV2,
		s.getProviderOwnedTargetgroups,
		s.getProviderOwnedSecurityGroups,
	}
}
