package framework

import (
	"context"
	"errors"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/kafkaqin/kube-scheduler/pkg/scheduler/apis/config"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework/parallelize"
	"math"
	"strings"
	"sync"
	"time"
)

type NodeScoreList []NodeScore
type NodeScore struct {
	Name  string
	Score int64
}

type NodeToStatusMap map[string]*Status

type NodePluginScores struct {
	Name       string
	Scores     PluginScore
	TotalScore int64
}

type PluginScore struct {
	Name  string
	Score int64
}

var codes = []string{"Success", "Error", "Unschedulable", "UnschedulableAndUnresolvable", "Wait", "Skip"}

func (c Code) String() string {
	return codes[c]
}

const (
	MaxNodeScore  int64 = 100
	minNodeScore        = 0
	MaxTotalScore int64 = math.MaxInt64
)

var PodsToActivateKey StateKey = "kubernetes.io/pods-to-activate"

type PodsToActivate struct {
	sync.Mutex
	Map map[string]*v1.Pod
}

func (s *PodsToActivate) Clone() StateData {
	return s
}

func NewPodsToActivate() *PodsToActivate {
	return &PodsToActivate{
		Map: make(map[string]*v1.Pod),
	}
}

type Status struct {
	code         Code
	reasons      []string
	err          error
	failedPlugin string
}

func (s *Status) WithError(err error) *Status {
	s.err = err
	return s
}

func (s *Status) Code() Code {
	if s == nil {
		return Success
	}
	return s.code
}

func (s *Status) Message() string {
	if s == nil {
		return ""
	}
	return strings.Join(s.reasons, ",")
}

func (s *Status) SetFailedPlugin(plugin string) {
	s.failedPlugin = plugin
}

func (s *Status) WithFailedPlugin(plugin string) *Status {
	s.SetFailedPlugin(plugin)
	return s
}

func (s *Status) FailedPlugin() string {
	return s.failedPlugin
}

func (s *Status) Reasons() []string {
	if s.err != nil {
		return append([]string{s.err.Error()}, s.reasons...)
	}
	return s.reasons
}

func (s *Status) AppendReason(reason string) {
	s.reasons = append(s.reasons, reason)
}

func (s *Status) IsSuccess() bool {
	return s.Code() == Success
}

func (s *Status) IsWait() bool {
	return s.Code() == Wait
}
func (s *Status) IsSkip() bool {
	return s.Code() == Skip
}

func (s *Status) IsUnschedulable() bool {
	return s.Code() == Unschedulable || s.Code() == UnschedulableAndUnresolvable
}

func (s *Status) AsError() error {
	if s.IsSuccess() || s.IsSkip() || s.IsWait() {
		return nil
	}
	if s.err != nil {
		return s.err
	}
	return errors.New(s.Message())
}

func (s *Status) Equal(x *Status) bool {
	if s == nil || x == nil {
		return s.IsSuccess() && x.IsSuccess()
	}
	if s.Code() != x.Code() {
		return false
	}
	if !cmp.Equal(s.err, x.err, cmpopts.EquateErrors()) {
		return false
	}
	if !cmp.Equal(s.reasons, x.Reasons()) {
		return false
	}
	return cmp.Equal(s.failedPlugin, x.failedPlugin)
}

func NewStatus(code Code, reasons ...string) *Status {
	s := &Status{
		code:    code,
		reasons: reasons,
	}
	return s
}

func AsStatus(err error) *Status {
	if err == nil {
		return nil
	}
	return &Status{
		code: Error,
		err:  err,
	}
}

type Code int

const (
	Success Code = iota
	Error
	Unschedulable
	UnschedulableAndUnresolvable
	Wait
	Skip
)

type WaitingPod interface {
	GetPod() *v1.Pod
	GetPendingPlugin() []string
	Allow(pluginName string)
	Reject(pluginName, msg string)
}

type Plugin interface {
	Name() string
}

type PreEnqueuePlugin interface {
	Plugin
	PreEnqueue(ctx context.Context, p *v1.Pod) *Status
}

type LessFunc func(podInfo1, podInfo2 *v1.Pod) bool

type QueueSortPlugin interface {
	Plugin
	Less(*QueuedPodInfo, *QueuedPodInfo) bool
}

type EnqueueExtensions interface {
	Plugin
	EventsToRegister() []ClusterEventWithHint
}

type PreFilterExtensions interface {
	Plugin
	RemovePod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToRemove *PodInfo, nodeInfo *NodeInfo) *Status
}
type PreFilterResult struct {
	NodeNames sets.Set[string]
}

func (p *PreFilterResult) AllNodes() bool {
	return p == nil || p.NodeNames == nil
}
func (p *PreFilterResult) Merge(in *PreFilterResult) *PreFilterResult {
	if p.AllNodes() && in.AllNodes() {
		return nil
	}

	r := PreFilterResult{}
	if p.AllNodes() {
		r.NodeNames = in.NodeNames.Clone()
		return &r
	}
	if in.AllNodes() {
		r.NodeNames = p.NodeNames.Clone()
		return &r
	}

	r.NodeNames = p.NodeNames.Intersection(in.NodeNames)
	return &r
}

type PreFilterPlugin interface {
	Plugin
	PreFilter(ctx context.Context, state *CycleState, pod *v1.Pod) (*PreFilterResult, *Status)
	PreFilterExtensions() PreFilterExtensions
}

type FilterPlugin interface {
	Plugin
	Filter(ctx context.Context, state *CycleState, pod *v1.Pod, nodeInfo *NodeInfo) *Status
}
type NominatingMode int

const (
	ModeNoop NominatingMode = iota
	ModeOverride
)

type NominatingInfo struct {
	NominatedNodeName string
	NominatingMode    NominatingMode
}

type PostFilterResult struct {
	*NominatingInfo
}

type PostFilterPlugin interface {
	Plugin
	PostFilter(ctx context.Context, state *CycleState, pod *v1.Pod, filteredNodeStatusMap NodeToStatusMap) (*PostFilterResult, *Status)
}

func NewPostFilterResultWithNominatedNode(name string) *PostFilterResult {
	return &PostFilterResult{
		NominatingInfo: &NominatingInfo{
			NominatedNodeName: name,
			NominatingMode:    ModeOverride,
		},
	}
}

type PreScorePlugin interface {
	Plugin
	PreScore(ctx context.Context, state *CycleState, pod *v1.Pod, nodes []*v1.Node) *Status
}

type ScoreExtensions interface {
	NormalizeScore(ctx context.Context, state *CycleState, pod *v1.Pod, scores NodeScoreList) *Status
}

type ScorePlugin interface {
	Plugin
	Score(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) (int64, *Status)
	ScoreExtensions() ScoreExtensions
}
type ReservePlugin interface {
	Plugin
	Reserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	Unreserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)
}

type PreBindPlugin interface {
	Plugin
	PreBind(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
}

type PostBindPlugin interface {
	Plugin
	PostBind(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)
}

type PodNominator interface {
	// AddNominatedPod adds the given pod to the nominator or
	// updates it if it already exists.
	AddNominatedPod(logger klog.Logger, pod *PodInfo, nominatingInfo *NominatingInfo)
	// DeleteNominatedPodIfExists deletes nominatedPod from internal cache. It's a no-op if it doesn't exist.
	DeleteNominatedPodIfExists(pod *v1.Pod)
	// UpdateNominatedPod updates the <oldPod> with <newPod>.
	UpdateNominatedPod(logger klog.Logger, oldPod *v1.Pod, newPodInfo *PodInfo)
	// NominatedPodsForNode returns nominatedPods on the given node.
	NominatedPodsForNode(nodeName string) []*PodInfo
}

type PluginsRunner interface {
	// RunPreScorePlugins runs the set of configured PreScore plugins. If any
	// of these plugins returns any status other than "Success", the given pod is rejected.
	RunPreScorePlugins(context.Context, *CycleState, *v1.Pod, []*v1.Node) *Status
	// RunScorePlugins runs the set of configured scoring plugins.
	// It returns a list that stores scores from each plugin and total score for each Node.
	// It also returns *Status, which is set to non-success if any of the plugins returns
	// a non-success status.
	RunScorePlugins(context.Context, *CycleState, *v1.Pod, []*v1.Node) ([]NodePluginScores, *Status)
	// RunFilterPlugins runs the set of configured Filter plugins for pod on
	// the given node. Note that for the node being evaluated, the passed nodeInfo
	// reference could be different from the one in NodeInfoSnapshot map (e.g., pods
	// considered to be running on the node could be different). For example, during
	// preemption, we may pass a copy of the original nodeInfo object that has some pods
	// removed from it to evaluate the possibility of preempting them to
	// schedule the target pod.
	RunFilterPlugins(context.Context, *CycleState, *v1.Pod, *NodeInfo) *Status
	// RunPreFilterExtensionAddPod calls the AddPod interface for the set of configured
	// PreFilter plugins. It returns directly if any of the plugins return any
	// status other than Success.
	RunPreFilterExtensionAddPod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToAdd *PodInfo, nodeInfo *NodeInfo) *Status
	// RunPreFilterExtensionRemovePod calls the RemovePod interface for the set of configured
	// PreFilter plugins. It returns directly if any of the plugins return any
	// status other than Success.
	RunPreFilterExtensionRemovePod(ctx context.Context, state *CycleState, podToSchedule *v1.Pod, podInfoToRemove *PodInfo, nodeInfo *NodeInfo) *Status
}

type PermitPlugin interface {
	Plugin
	PrePermit(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) (*Status, time.Duration)
}

type BindPlugin interface {
	Plugin
	Bind(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
}
type Handle interface {
	PodNominator
	PluginsRunner
	SnapshotSharedLister() SharedLister
	IterateOverWaitingPods(callback func(pod WaitingPod))

	GetWaitingPod() WaitingPod
	RejectWaitingPod(uid types.UID) bool
	ClientSet() clientset.Interface
	KubeConfig() *restclient.Config
	EventRecorder() events.EventRecorder
	SharedInformerFactory() informers.SharedInformerFactory
	RunFilterPluginsWithNominatedPods(ctx context.Context, state *CycleState, pod *v1.Pod, info *NodeInfo) *Status
	Extenders() []Extender
	Parallelizer() parallelize.Parallelizer
}
type Framework interface {
	Handle
	PreEnqueuePlugins() []PreEnqueuePlugin
	EnqueueExtensions() []EnqueueExtensions
	QueueSortFunc() LessFunc
	RunPreFilterPlugins(ctx context.Context, state *CycleState, pod *v1.Pod) (PreFilterResult, *Status)
	RunPostFilterPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, filteredNodeStatusMap NodeToStatusMap) (PostFilterResult, *Status)

	RunPreBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	RunPostBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)

	RunReservePluginsReserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	RunReservePluginsUnreserve(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string)
	RunPermitPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	WaitOnPermit(ctx context.Context, pod *v1.Pod) *Status
	RunBindPlugins(ctx context.Context, state *CycleState, pod *v1.Pod, nodeName string) *Status
	HasFilterPlugins() bool
	HasPostFilterPlugins() bool
	HasScorePlugins() bool
	ListPlugins() *config.Plugins
	ProfileName() string
	PercentageOfNodesToScore() *int32
	SetPodNominator(podNominator PodNominator)
}
