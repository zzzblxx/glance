package e1003

import (
	"github.com/zzzblxx/glance/internal/devices"
	"github.com/zzzblxx/glance/internal/protocol"
)

const RendererVersion = "glance-3"

func Profile() devices.Profile {
	return devices.Profile{ID: "e1003", RendererVersion: RendererVersion,
		Protocol: protocol.Protocol, BaudRate: 115200, Width: 1404, Height: 1872,
		GrayLevels: 16, MaxPages: protocol.MaxPages}
}
