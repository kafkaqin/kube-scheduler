package config

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	componentbaseconfig "k8s.io/component-base/config"
	"math"
)

const (
	SchedulerPolicyConfigMapKey = "policy.cfg"
	DefaultKubeSchedulerPort    = 10259
)

type KubeSchedulerConfiguration struct {
	metav1.TypeMeta
	Parallelism        int32
	LeaderElection     componentbaseconfig.LeaderElectionConfiguration
	ClientConnection   componentbaseconfig.ClientConnectionConfiguration
	HealthzBindAddress string
	MetricsBindAddress string
	componentbaseconfig.DebuggingConfiguration
	PercentageOfNodesToScore int32
	PodInitialBackoffSeconds int32
	PodMaxBackoffSeconds     int32
	Profiles                 []KubeSchedulerProfile
	Extenders                []Extender
	DelayCacheUntilActive    bool
}

type Extender struct {
	URLPrefix        string
	FilterVerb       string
	PreemptVerb      string
	PriorityVerb     string
	Weight           int64
	BindVerb         string
	EnableHTTPS      bool
	TLSConfig        *ExtenderTLSConfig
	HTTPTimeout      metav1.Duration
	NodeCacheCapable bool
	ManageResources  []ExtenderManagedResource
	Ignorable        bool
}

type ExtenderManagedResource struct {
	// Name is the extended resource name.
	Name string
	// IgnoredByScheduler indicates whether kube-scheduler should ignore this
	// resource when applying predicates.
	IgnoredByScheduler bool
}

// ExtenderTLSConfig contains settings to enable TLS with extender
type ExtenderTLSConfig struct {
	// Server should be accessed without verifying the TLS certificate. For testing only.
	Insecure bool
	// ServerName is passed to the server for SNI and is used in the client to check server
	// certificates against. If ServerName is empty, the hostname used to contact the
	// server is used.
	ServerName string

	// Server requires TLS client certificate authentication
	CertFile string
	// Server requires TLS client certificate authentication
	KeyFile string
	// Trusted root certificates for server
	CAFile string

	// CertData holds PEM-encoded bytes (typically read from a client certificate file).
	// CertData takes precedence over CertFile
	CertData []byte
	// KeyData holds PEM-encoded bytes (typically read from a client certificate key file).
	// KeyData takes precedence over KeyFile
	KeyData []byte `datapolicy:"security-key"`
	// CAData holds PEM-encoded bytes (typically read from a root certificates bundle).
	// CAData takes precedence over CAFile
	CAData []byte
}

type KubeSchedulerProfile struct {
	SchedulerName            string
	PercentageOfNodesToScore int32
	Plugins                  *Plugins
	PluginConfig             []PluginConfig
}
type PluginConfig struct {
	// Name defines the name of plugin being configured
	Name string
	// Args defines the arguments passed to the plugins at the time of initialization. Args can have arbitrary structure.
	Args runtime.Object
}

type PluginSet struct {
	Enabled  []Plugin
	Disabled []Plugin
}

type Plugin struct {
	Name   string
	Weight int32
}
type Plugins struct {
	PreEnqueue PluginSet
	QueueSort  PluginSet
	PreFilter  PluginSet
	Filter     PluginSet
	PostFilter PluginSet
	PreScore   PluginSet
	Score      PluginSet
	Reserve    PluginSet
	Permit     PluginSet
	PreBind    PluginSet
	Bind       PluginSet
	PostBind   PluginSet
	MultiPoint PluginSet
}

const (
	DefaultPercentageOfNodesToScore       = 0
	MaxCustomPriorityScore          int64 = 10
	MaxTotalScore                         = math.MaxInt64
	MaxWeight                             = MaxTotalScore / MaxCustomPriorityScore
)

func (p *Plugins) Names() []string {
	if p == nil {
		return []string{}
	}
	extension := []PluginSet{
		p.PreEnqueue,
		p.PreFilter,
		p.Filter,
		p.PostFilter,
		p.Reserve,
		p.PreScore,
		p.Score,
		p.PreBind,
		p.Bind,
		p.PostBind,
		p.Permit,
		p.QueueSort,
	}

	n := sets.New[string]()
	for _, e := range extension {
		for _, pg := range e.Enabled {
			n.Insert(pg.Name)
		}
	}

	return sets.List(n)
}
