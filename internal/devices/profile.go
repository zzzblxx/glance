// Package devices describes supported hardware independently of the transport.
package devices

type Profile struct {
	ID              string
	RendererVersion string
	Protocol        int
	BaudRate        int
	Width           int
	Height          int
	GrayLevels      int
	MaxPages        int
}
