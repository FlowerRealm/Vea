package repository

import "errors"

// 通用仓储错误
var (
	// ErrNotFound 实体不存在
	ErrNotFound = errors.New("entity not found")

	// ErrAlreadyExists 实体已存在
	ErrAlreadyExists = errors.New("entity already exists")

	// ErrInvalidID ID 无效
	ErrInvalidID = errors.New("invalid entity ID")

	// ErrInvalidData 数据无效
	ErrInvalidData = errors.New("invalid entity data")
)

// FRouter 相关错误
var (
	ErrFRouterNotFound = errors.New("frouter not found")
)

// Node 相关错误
var (
	ErrNodeNotFound = errors.New("node not found")
)

// 配置相关错误
var (
	ErrConfigNotFound = errors.New("config not found")
)

// Geo 资源相关错误
var (
	ErrGeoNotFound = errors.New("geo resource not found")
)

// 组件相关错误
var (
	ErrComponentNotFound = errors.New("component not found")
)
