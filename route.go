//go:build windows

// Package winroute 提供了一个现代化、用户友好的接口来操作 Windows 路由表。
// 它建立在 wireguard/winipcfg 之上，封装了底层的复杂性，
// 提供了信息聚合和便捷的操作功能。
package winroute

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// ErrNotFound 表示未找到指定的路由或接口。
var ErrNotFound = errors.New("not found")

// ErrAmbiguousMatch 表示过滤器条件匹配了多个路由，无法确定要操作的单个目标。
var ErrAmbiguousMatch = errors.New("filter criteria matched multiple routes")

// ---- GetRoutes: 查询路由 ----

// FilterOption 是一个函数类型，用于定义对路由的过滤条件。
type FilterOption func(r *Route) bool

// WithDestinationPrefix 创建一个过滤器，仅保留目标网段完全匹配的路由。
func WithDestinationPrefix(prefix netip.Prefix) FilterOption {
	return func(r *Route) bool {
		return r.Destination == prefix
	}
}

// WithInterfaceIndex 创建一个过滤器，仅保留通过指定接口索引的路由。
func WithInterfaceIndex(index uint32) FilterOption {
	return func(r *Route) bool {
		return r.Interface.Index == index
	}
}

// WithInterfaceAlias 创建一个过滤器，仅保留通过指定接口别名（不区分大小写）的路由。
func WithInterfaceAlias(alias string) FilterOption {
	return func(r *Route) bool {
		return strings.EqualFold(r.Interface.Alias, alias)
	}
}

// WithMetric 创建一个过滤器，仅保留Metric等于指定值的路由。
func WithMetric(metric uint32) FilterOption {
	return func(r *Route) bool {
		return r.Metric == metric
	}
}

// GetRoutes 获取系统路由表，并可选择性地应用一个或多个过滤器。
func GetRoutes(filters ...FilterOption) ([]*Route, error) {
	// 1. 构建接口缓存，以便后面快速查找接口信息
	cache, err := newInterfaceCache()
	if err != nil {
		return nil, fmt.Errorf("failed to build interface cache: %w", err)
	}

	// 2. 从 winipcfg 获取基础路由表
	baseRoutes, err := winipcfg.GetIPForwardTable2(windows.AF_UNSPEC)
	if err != nil {
		return nil, fmt.Errorf("failed to get base routing table: %w", err)
	}

	// 3. 聚合信息并执行过滤
	routes := make([]*Route, 0, len(baseRoutes))
	for i := range baseRoutes {
		baseRoute := &baseRoutes[i]

		// 从缓存中查找此路由关联的接口
		iface, ok := cache.byLUID[baseRoute.InterfaceLUID]
		if !ok {
			// 接口可能已不存在或不可用，跳过这条路由
			continue
		}

		// 构建我们自己的 "富对象" Route
		route := &Route{
			Destination: baseRoute.DestinationPrefix.Prefix(),
			NextHop:     baseRoute.NextHop.Addr(),
			Interface:   iface,
			Metric:      baseRoute.Metric,
			Protocol:    baseRoute.Protocol,
			Origin:      baseRoute.Origin,
		}

		// 应用所有过滤器
		matches := true
		for _, filter := range filters {
			if !filter(route) {
				matches = false
				break
			}
		}

		if matches {
			routes = append(routes, route)
		}
	}

	return routes, nil
}

// ---- AddRoute: 增加路由 ----

// AddRoute 添加一条新路由。
// ifaceIndex 是index。
// 注意：通过此 API 添加的路由在系统重启后不会保留（非持久化）。
func AddRoute(destination netip.Prefix, nextHop netip.Addr, ifaceIndex uint32, metric uint32) error {
	luid, err := winipcfg.LUIDFromIndex(ifaceIndex)
	if err != nil {
		return fmt.Errorf("failed to convert interface index to LUID: %w", err)
	}
	// 填充 winipcfg 需要的结构体
	if err := luid.AddRoute(destination, nextHop, metric); err != nil {
		// 检查是否因为路由已存在而失败
		if errors.Is(err, windows.ERROR_OBJECT_ALREADY_EXISTS) {
			return fmt.Errorf("route to %s already exists: %w", destination, err)
		}
		return fmt.Errorf("failed to create route: %w", err)
	}

	return nil
}

// ---- DeleteRoute: 删除路由 ----

// DeleteRoute 删除一条精确匹配的路由。
// 所有参数（目标、下一跳、接口）都必须匹配才能成功删除。
func DeleteRoute(destination netip.Prefix, nextHop netip.Addr, ifaceIndex uint32) error {
	luid, err := winipcfg.LUIDFromIndex(ifaceIndex)
	if err != nil {
		return fmt.Errorf("failed to convert interface index to LUID: %w", err)
	}

	if err := luid.DeleteRoute(destination, nextHop); err != nil {
		// 检查是否因为路由不存在而失败
		if errors.Is(err, windows.ERROR_NOT_FOUND) {
			return fmt.Errorf("route to %s not found: %w", destination, ErrNotFound)
		}
		return fmt.Errorf("failed to delete route: %w", err)
	}

	return nil
}

// ---- DeleteRoutes: 批量删除路由 ----

// ErrorAction 定义了在批量操作中遇到错误时的行为。
type ErrorAction int

const (
	// ErrorActionContinue 表示即使发生错误，也继续处理其余项目。
	// 这是默认行为。所有错误将被收集并一起返回。
	ErrorActionContinue ErrorAction = iota
	// ErrorActionStop 表示在遇到第一个错误时立即停止操作。
	ErrorActionStop
)

// extractRouteParameters 从选项列表中解析出过滤器和错误处理行为。
func extractRouteParameters(opts ...any) ([]FilterOption, ErrorAction, error) {
	var filters []FilterOption
	errorAction := ErrorActionContinue // 默认行为

	for _, opt := range opts {
		switch o := opt.(type) {
		case FilterOption:
			filters = append(filters, o)
		case ErrorAction:
			errorAction = o
		default:
			return nil, 0, fmt.Errorf("unsupported option type: %T", o)
		}
	}

	return filters, errorAction, nil
}

// DeleteRoutes 按照一组过滤器和行为选项删除路由。
//
// opts 参数可以接收两种类型的选项：
//   - FilterOption: 用于指定要删除哪些路由 (例如 WithDestinationPrefix, WithInterfaceAlias)。
//   - ErrorAction: 用于配置删除过程的行为 (ErrorActionContinue 或 ErrorActionStop)。
//
// 默认行为是“继续执行并聚合所有错误”（ErrorActionContinue）。
//
// 返回值:
//   - partialErrs ([]error): 在 ContinueOnError 模式下，收集所有删除失败的错误。如果全部成功，则为 nil。
//   - err (error): 操作过程中的致命错误（如无法获取路由列表）。在 ContinueOnError 模式下，即使有部分删除失败，此错误也为 nil。
func DeleteRoutes(opts ...any) (partialErrs []error, err error) {
	filters, errorAction, err := extractRouteParameters(opts...)
	if err != nil {
		return nil, err
	}

	routes, err := GetRoutes(filters...)
	if err != nil {
		return nil, fmt.Errorf("failed to find routes for deletion: %w", err)
	}

	if len(routes) == 0 {
		return nil, nil
	}

	// deletedCount 和 partialErrs 已在命名返回值中声明
	for _, route := range routes {
		if delErr := route.Delete(); delErr != nil {
			wrappedErr := fmt.Errorf("failed to delete route (dest: %s, iface: %s): %w", route.Destination, route.Interface.Alias, delErr)
			if errorAction == ErrorActionStop {
				// 对于 StopOnError，我们将错误作为主错误返回
				return nil, wrappedErr
			}
			// 对于 ContinueOnError，我们将错误添加到部分错误列表中
			partialErrs = append(partialErrs, wrappedErr)
		}
	}

	// 在 ContinueOnError 模式下，即使 partialErrs 不为空，主错误也为 nil
	return partialErrs, nil
}
