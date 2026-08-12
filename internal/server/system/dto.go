package system

import (
	"net"

	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func endpointInfo(remoteAddr string) api.EndPointInfo {
	ip := remoteIP(remoteAddr)
	local := ip != nil && ip.IsLoopback()

	return api.EndPointInfo{
		IsLocal:     apiutil.Ptr(local),
		IsInNetwork: apiutil.Ptr(local || (ip != nil && (ip.IsPrivate() || ip.IsLinkLocalUnicast()))),
	}
}

func remoteIP(remoteAddr string) net.IP {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}

	return net.ParseIP(host)
}
