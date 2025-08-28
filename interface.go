package winroute

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// ---- 辅助工具：接口缓存和查询 ----

// interfaceCache 用于在单次操作中缓存接口信息，避免重复的API调用。
type interfaceCache struct {
	byLUID  map[winipcfg.LUID]*Interface
	byIndex map[uint32]*Interface
	byAlias map[string]*Interface
}

// newInterfaceCache 通过查询系统API来构建接口信息的完整缓存。
func newInterfaceCache() (*interfaceCache, error) {
	// 使用 winipcfg 获取大部分接口信息
	adapters, err := winipcfg.GetAdaptersAddresses(windows.AF_UNSPEC, windows.GAA_FLAG_INCLUDE_PREFIX)
	if err != nil {
		return nil, fmt.Errorf("failed to get adapters addresses: %w", err)
	}

	cache := &interfaceCache{
		byLUID:  make(map[winipcfg.LUID]*Interface, len(adapters)),
		byIndex: make(map[uint32]*Interface, len(adapters)),
		byAlias: make(map[string]*Interface, len(adapters)),
	}

	for _, adapter := range adapters {
		// adapter.FriendlyName() 通常就是我们需要的接口 "别名" (Alias)，
		// 例如 "以太网" 或 "Wi-Fi"。直接使用它可以简化代码。
		iface := &Interface{
			Index:       adapter.IfIndex,
			LUID:        adapter.LUID,
			Alias:       adapter.FriendlyName(),
			Description: adapter.Description(),
		}

		cache.byLUID[iface.LUID] = iface
		cache.byIndex[iface.Index] = iface
		// 别名可能不唯一，但为了方便查询，我们先简单处理。
		// 在实际应用中，如果别名冲突，findInterface应该返回错误。
		if _, exists := cache.byAlias[strings.ToLower(iface.Alias)]; !exists {
			cache.byAlias[strings.ToLower(iface.Alias)] = iface
		}
	}
	return cache, nil
}

// findInterface 根据标识符（可以是Index或Alias）在缓存中查找接口。
func (c *interfaceCache) findInterface(identifier string) (*Interface, error) {
	// 尝试按 Index 解析
	if index, err := strconv.ParseUint(identifier, 10, 32); err == nil {
		if iface, ok := c.byIndex[uint32(index)]; ok {
			return iface, nil
		}
	}

	// 尝试按 Alias 查找
	if iface, ok := c.byAlias[strings.ToLower(identifier)]; ok {
		return iface, nil
	}

	return nil, fmt.Errorf("interface '%s' not found: %w", identifier, ErrNotFound)
}
