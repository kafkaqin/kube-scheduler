package runtime

import (
	"github.com/kafkaqin/kube-scheduler/pkg/scheduler/apis/config"
	"github.com/kafkaqin/kube-scheduler/pkg/scheduler/framework"
	"github.com/kafkaqin/kube-scheduler/pkg/scheduler/framework/parallelize"
	"k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/events"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/metrics"
	"time"
)

const (
	MaxTimeout = 15 * time.Minute
)

type frameworkImpl struct {
	registry                 Registry
	snapshotSharedLister     framework.SharedLister
	waitingPods              *waitingPodsMap
	scorePluginWeight        map[string]int
	preEnqueuePlugins        []framework.PreEnqueuePlugin
	enqueueExtensions        []framework.EnqueueExtensions
	queueSortPlugins         []framework.QueueSortPlugin
	preFilterPlugins         []framework.PreFilterPlugin
	filterPlugins            []framework.FilterPlugin
	postFilterPlugins        []framework.PostFilterPlugin
	preScorePlugins          []framework.PreScorePlugin
	scorePlugins             []framework.ScorePlugin
	reservePlugins           []framework.ReservePlugin
	preBindPlugins           []framework.PreBindPlugin
	bindPlugins              []framework.BindPlugin
	postBindPlugins          []framework.PostBindPlugin
	permitPlugins            []framework.PermitPlugin
	clientSet                clientset.Interface
	kubeConfig               *restclient.Config
	eventRecorder            events.EventRecorder
	informerFactory          informers.SharedInformerFactory
	logger                   klog.Logger
	metricsRecorder          *metrics.MetricRecorder
	profileName              string
	percentageOfNodesToScore *int32
	extenders                []framework.Extender
	framework.PodNominator
	parallelizer parallelize.Parallelizer
}

type extensionPoint struct {
	plugins  *config.PluginSet
	slicePtr interface{}
}

func (f *frameworkImpl) getExtensionPoints(plugins *config.Plugins) []extensionPoint {
	return []extensionPoint{
		{&plugins.PreFilter, &f.preFilterPlugins},
		{&plugins.Filter, &f.filterPlugins},
		{&plugins.PostFilter, &f.postFilterPlugins},
		{&plugins.Reserve, &f.reservePlugins},
		{&plugins.PreScore, &f.preScorePlugins},
		{&plugins.Score, &f.scorePlugins},
		{&plugins.PreBind, &f.preBindPlugins},
		{&plugins.Bind, &f.bindPlugins},
		{&plugins.PostBind, &f.postBindPlugins},
		{&plugins.Permit, &f.permitPlugins},
		{&plugins.PreEnqueue, &f.preEnqueuePlugins},
		{&plugins.QueueSort, &f.queueSortPlugins},
	}
}

func (f *frameworkImpl) Extenders() []framework.Extender {
	return f.extenders
}

type frameworkOptions struct {
	componentConfigVersion string
	clientSet              clientset.Interface
	kubeConfig             *restclient.Config
	eventRecorder          events.EventRecorder
	informerFactory        informers.SharedInformerFactory
	snapshotSharedLister   framework.SharedLister
	metricsRecorder        *metrics.MetricAsyncRecorder
	podNominator           framework.PodNominator
	extenders              []framework.Extender
	captureProfile         CaptureProfile
	parallelizer           parallelize.Parallelizer
	logger                 *klog.Logger
}
type Option func(*frameworkOptions)

func WithComponentConfigVersion(componentConfigVersion string) Option {
	return func(o *frameworkOptions) {
		o.componentConfigVersion = componentConfigVersion
	}
}

// WithClientSet sets clientSet for the scheduling frameworkImpl.
func WithClientSet(clientSet clientset.Interface) Option {
	return func(o *frameworkOptions) {
		o.clientSet = clientSet
	}
}

// WithKubeConfig sets kubeConfig for the scheduling frameworkImpl.
func WithKubeConfig(kubeConfig *restclient.Config) Option {
	return func(o *frameworkOptions) {
		o.kubeConfig = kubeConfig
	}
}

// WithEventRecorder sets clientSet for the scheduling frameworkImpl.
func WithEventRecorder(recorder events.EventRecorder) Option {
	return func(o *frameworkOptions) {
		o.eventRecorder = recorder
	}
}

// WithInformerFactory sets informer factory for the scheduling frameworkImpl.
func WithInformerFactory(informerFactory informers.SharedInformerFactory) Option {
	return func(o *frameworkOptions) {
		o.informerFactory = informerFactory
	}
}

// WithSnapshotSharedLister sets the SharedLister of the snapshot.
func WithSnapshotSharedLister(snapshotSharedLister framework.SharedLister) Option {
	return func(o *frameworkOptions) {
		o.snapshotSharedLister = snapshotSharedLister
	}
}

// WithPodNominator sets podNominator for the scheduling frameworkImpl.
func WithPodNominator(nominator framework.PodNominator) Option {
	return func(o *frameworkOptions) {
		o.podNominator = nominator
	}
}

// WithExtenders sets extenders for the scheduling frameworkImpl.
func WithExtenders(extenders []framework.Extender) Option {
	return func(o *frameworkOptions) {
		o.extenders = extenders
	}
}

// WithParallelism sets parallelism for the scheduling frameworkImpl.
func WithParallelism(parallelism int) Option {
	return func(o *frameworkOptions) {
		o.parallelizer = parallelize.NewParallelizer(parallelism)
	}
}

type CaptureProfile func(config.KubeSchedulerProfile)

func WithCaptureProfile(c CaptureProfile) Option {
	return func(o *frameworkOptions) {
		o.captureProfile = c
	}
}

// WithMetricsRecorder sets metrics recorder for the scheduling frameworkImpl.
func WithMetricsRecorder(r *metrics.MetricAsyncRecorder) Option {
	return func(o *frameworkOptions) {
		o.metricsRecorder = r
	}
}

// WithLogger overrides the default logger from k8s.io/klog.
func WithLogger(logger klog.Logger) Option {
	return func(o *frameworkOptions) {
		o.logger = &logger
	}
}
