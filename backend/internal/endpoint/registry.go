package endpoint

import (
	"fmt"
	"sync"
)

// Registry 端点注册中心
type Registry struct {
	endpoints map[string]Endpoint // type -> endpoint
	mu        sync.RWMutex
}

var (
	globalRegistry = NewRegistry()
)

// NewRegistry 创建端点注册中心
func NewRegistry() *Registry {
	return &Registry{
		endpoints: make(map[string]Endpoint),
	}
}

// Register 注册端点
func (r *Registry) Register(ep Endpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints[ep.GetType()] = ep
}

// Get 根据类型获取端点
func (r *Registry) Get(endpointType string) (Endpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ep, ok := r.endpoints[endpointType]
	if !ok {
		return nil, fmt.Errorf("endpoint type not found: %s", endpointType)
	}
	return ep, nil
}

// GetAll 获取所有端点
func (r *Registry) GetAll() []Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Endpoint, 0, len(r.endpoints))
	for _, ep := range r.endpoints {
		result = append(result, ep)
	}
	return result
}

// GetAllInfo 获取所有端点信息
func (r *Registry) GetAllInfo() []EndpointInfo {
	endpoints := r.GetAll()
	result := make([]EndpointInfo, 0, len(endpoints))
	for _, ep := range endpoints {
		result = append(result, ToInfo(ep))
	}
	return result
}

// Exists 检查端点类型是否存在
func (r *Registry) Exists(endpointType string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.endpoints[endpointType]
	return ok
}

// GetGlobalRegistry 获取全局注册中心
func GetGlobalRegistry() *Registry {
	return globalRegistry
}

// Register 注册端点到全局注册中心
func Register(ep Endpoint) {
	globalRegistry.Register(ep)
}

// Get 从全局注册中心获取端点
func Get(endpointType string) (Endpoint, error) {
	return globalRegistry.Get(endpointType)
}

// GetAll 从全局注册中心获取所有端点
func GetAll() []Endpoint {
	return globalRegistry.GetAll()
}

// GetAllInfo 从全局注册中心获取所有端点信息
func GetAllInfo() []EndpointInfo {
	return globalRegistry.GetAllInfo()
}
