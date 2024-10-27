package framework

import (
	v1 "k8s.io/api/core/v1"
	extenderv1 "k8s.io/kube-scheduler/extender/v1"
)

type Extender interface {
	Name() string
	Filter(pod *v1.Pod, nodes []*v1.Node) (filteNodes []*v1.Node, failedNodeMap extenderv1.FailedNodesMap, failedAndUnresolvable extenderv1.FailedNodesMap, err error)
	Prioritize(pod *v1.Pod, nodes []v1.Node) (hostPriorities *extenderv1.HostPriorityList, weight int64, err error)
	Bind(binding *v1.Binding) error
	IsBinder() bool
	IsInterested(pod *v1.Pod) bool
	ProcessPreemption(pod *v1.Pod, nodeNameToVictims map[string]*extenderv1.Victims, nodeInfos NodeInfoLister) (map[string]*extenderv1.Victims, error)

	SupportPreemption() bool
	IsIgnorable() bool
}
