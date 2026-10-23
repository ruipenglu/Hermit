// Package app 暴露给 Wails 前端的服务层，核心逻辑在 internal/core。
package app

// Service 是前端可调用的全部后端能力。
type Service struct{}

// New 创建服务实例。
func New() *Service { return &Service{} }

// Version 返回应用版本号。
func (s *Service) Version() string { return "1.1.0" }
