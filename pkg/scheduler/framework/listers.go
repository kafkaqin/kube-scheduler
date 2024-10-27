package framework

type NodeInfoLister interface {
	List() ([]*NodeInfo, error)
	HavePodsWithAffinityList() ([]*NodeInfo, error)
	HavePodsWithRequiredAntiAffinityList() ([]*NodeInfo, error)
	Get(nodeName string) (*NodeInfo, error)
}

type StorageInfoLister interface {
	IsPVCUsedByPods(key string) (bool, error)
}
type SharedLister interface {
	NodeInfos() NodeInfoLister
	StorageInfos() StorageInfoLister
}
