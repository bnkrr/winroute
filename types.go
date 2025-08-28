package winroute

import (
	"net/netip"

	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// Interface 代表一个网络接口的聚合信息。
type Interface struct {
	Index       uint32
	LUID        winipcfg.LUID
	Alias       string // 用户友好的名字, e.g., "以太网"
	Description string // 接口描述, e.g., "Realtek PCIe GbE Family Controller"
}

// Route 代表一条完整的、信息丰富的路由。
type Route struct {
	Destination netip.Prefix
	NextHop     netip.Addr
	Interface   *Interface // 路由所使用的接口
	Metric      uint32
	Protocol    winipcfg.RouteProtocol
	Origin      winipcfg.RouteOrigin
}

func (r *Route) Delete() error {
	return r.Interface.LUID.DeleteRoute(r.Destination, r.NextHop)
}
