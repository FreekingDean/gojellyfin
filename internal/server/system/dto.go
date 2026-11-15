package system

import (
	"net"

	"github.com/FreekingDean/gojellyfin/internal/auth"
	"github.com/FreekingDean/gojellyfin/internal/server/api"
	"github.com/FreekingDean/gojellyfin/internal/server/apiutil"
)

func endpointInfo(authorization auth.Authorization) api.EndPointInfo {
	ip := remoteIP(authorization.RemoteAddr)
	server := remoteIP(authorization.LocalAddr)
	local := ip != nil && (ip.IsLoopback() || ip.Equal(server))

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
