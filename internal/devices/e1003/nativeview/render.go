// Package nativeview uses the device's renderer for local preview only.
// Its image bytes never enter the USB publisher.
package nativeview

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/protocol"
	"github.com/zzzblxx/glance/internal/render"
)

type Renderer struct{ binary, font string }

func New(binary string) (*Renderer, error) {
	if binary == "" {
		self, err := os.Executable()
		if err != nil {
			return nil, err
		}
		binary = filepath.Join(filepath.Dir(self), "glance-render")
	}
	font := filepath.Join(filepath.Dir(binary), "unifont16.bin")
	if _, err := os.Stat(binary); err != nil {
		return nil, errors.New("shared preview helper missing; run scripts/build.sh or reinstall the complete package")
	}
	if _, err := os.Stat(font); err != nil {
		return nil, errors.New("shared preview font missing")
	}
	return &Renderer{binary, font}, nil
}
func (r *Renderer) Close()                                         {}
func (r *Renderer) Render(d model.Dashboard) (render.Frame, error) { return r.RenderPage(d, 0) }
func (r *Renderer) RenderPage(d model.Dashboard, page int) (render.Frame, error) {
	b, _, err := protocol.Encode(d)
	if err != nil {
		return render.Frame{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.binary, r.font, strconv.Itoa(page))
	cmd.Stdin = bytes.NewReader(b)
	out, err := cmd.Output()
	if err != nil {
		return render.Frame{}, errors.New("device renderer rejected snapshot")
	}
	return render.DecodeNative(out)
}
