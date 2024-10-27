package feature

type Features struct {
	EnableDynamicResourceAllocation              bool
	EnableReadWriteOncePod                       bool
	EnableVolumeCapacityPriority                 bool
	EnableMinDomainsInPodTopologySpread          bool
	EnableNodeInclusionPolicyInPodTopologySpread bool
	EnableMatchLabelKeysInPodTopologySpread      bool
	EnablePodSchedulingReadiness                 bool
	EnablePodDisruptionConditions                bool
	EnableInPlacePodVerticalScaling              bool
	EnableSidecarContainers                      bool
}
