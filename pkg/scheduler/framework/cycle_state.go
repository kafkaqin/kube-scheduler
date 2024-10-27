package framework

import (
	"errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"sync"
)

var (
	ErrorNotFound = errors.New("not found")
)

type StateData interface {
	Clone() StateData
}

type StateKey string
type CycleState struct {
	storage             sync.Map
	recordPluginMetrics bool
	SkipFilterPlugins   sets.Set[string]
	SkipScorePlugins    sets.Set[string]
}

func NewCycleState() *CycleState {
	return &CycleState{}
}

func (cs *CycleState) ShouldRecordPluginMetrics() bool {
	if cs == nil {
		return false
	}
	return cs.recordPluginMetrics
}

func (cs *CycleState) SetRecordPluginMetrics(flag bool) {
	if cs == nil {
		return
	}
	cs.recordPluginMetrics = flag

}

func (cs *CycleState) Clone() *CycleState {
	if cs == nil {
		return nil
	}
	copy := NewCycleState()
	cs.storage.Range(func(key, value interface{}) bool {
		copy.storage.Store(key, value.(StateData).Clone())
		return true
	})
	copy.recordPluginMetrics = cs.recordPluginMetrics
	copy.SkipFilterPlugins = cs.SkipFilterPlugins
	copy.SkipScorePlugins = cs.SkipScorePlugins
	return copy
}

func (cs *CycleState) Read(key StateKey) (StateData, error) {
	if value, ok := cs.storage.Load(key); ok {
		return value.(StateData), nil
	}
	return nil, ErrorNotFound
}
func (cs *CycleState) Write(key StateKey, value StateData) {
	cs.storage.Store(key, value)
}
func (cs *CycleState) Delete(key StateKey) {
	cs.storage.Delete(key)
}
