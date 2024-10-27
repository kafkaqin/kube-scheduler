package runtime

import (
	"fmt"
	"github.com/kafkaqin/kube-scheduler/pkg/scheduler/framework"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sync"
	"time"
)

type waitingPodsMap struct {
	pods  map[types.UID]*waitingPod
	mutex sync.RWMutex
}

func newWaitingPodsMap() *waitingPodsMap {
	return &waitingPodsMap{
		pods: make(map[types.UID]*waitingPod),
	}
}

func (m *waitingPodsMap) add(wp *waitingPod) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.pods[wp.GetPod().UID] = wp
}

func (m *waitingPodsMap) remove(uid types.UID) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	delete(m.pods, uid)
}

func (m *waitingPodsMap) get(uid types.UID) *waitingPod {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return m.pods[uid]
}

func (m *waitingPodsMap) iterate(callback func(pod framework.WaitingPod)) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	for _, v := range m.pods {
		callback(v)
	}
}

type waitingPod struct {
	pod            *v1.Pod
	pendingPlugins map[string]*time.Timer
	s              chan *framework.Status
	mu             sync.RWMutex
}

var _ framework.WaitingPod = &waitingPod{}

func newWaitingPod(pod *v1.Pod, pluginsMaxWaitTime map[string]time.Duration) *waitingPod {
	wp := &waitingPod{
		pod: pod,
		s:   make(chan *framework.Status, 1),
	}
	wp.pendingPlugins = make(map[string]*time.Timer, len(pluginsMaxWaitTime))
	wp.mu.Lock()
	defer wp.mu.Unlock()
	for k, v := range pluginsMaxWaitTime {
		plugin, waitTime := k, v
		wp.pendingPlugins[plugin] = time.AfterFunc(waitTime, func() {
			msg := fmt.Sprintf("rejected due to timeout after waiting %v	at plugin %v", waitTime, plugin)
			wp.Reject(plugin, msg)
		})
	}
	return wp
}
func (w *waitingPod) GetPod() *v1.Pod {
	return w.pod
}

// GetPendingPlugins returns a list of pending permit plugin's name.
func (w *waitingPod) GetPendingPlugin() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	plugins := make([]string, 0, len(w.pendingPlugins))
	for p := range w.pendingPlugins {
		plugins = append(plugins, p)
	}

	return plugins
}

// Allow declares the waiting pod is allowed to be scheduled by plugin pluginName.
// If this is the last remaining plugin to allow, then a success signal is delivered
// to unblock the pod.
func (w *waitingPod) Allow(pluginName string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if timer, exist := w.pendingPlugins[pluginName]; exist {
		timer.Stop()
		delete(w.pendingPlugins, pluginName)
	}

	// Only signal success status after all plugins have allowed
	if len(w.pendingPlugins) != 0 {
		return
	}

	// The select clause works as a non-blocking send.
	// If there is no receiver, it's a no-op (default case).
	select {
	case w.s <- framework.NewStatus(framework.Success, ""):
	default:
	}
}

// Reject declares the waiting pod unschedulable.
func (w *waitingPod) Reject(pluginName, msg string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	for _, timer := range w.pendingPlugins {
		timer.Stop()
	}

	// The select clause works as a non-blocking send.
	// If there is no receiver, it's a no-op (default case).
	select {
	case w.s <- framework.NewStatus(framework.Unschedulable, msg).WithFailedPlugin(pluginName):
	default:
	}
}
