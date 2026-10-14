package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"github.com/zzzblxx/glance/internal/companion"
	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/devices/e1003"
	"github.com/zzzblxx/glance/internal/devices/e1003/nativeview"
	"github.com/zzzblxx/glance/internal/model"
	"github.com/zzzblxx/glance/internal/server"
	"github.com/zzzblxx/glance/internal/storage"
	"github.com/zzzblxx/glance/internal/transport/serial"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var version = "1.0.0-dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: glance init-config | migrate-config | serve | preview | version")
	}
	if os.Args[1] == "version" {
		fmt.Printf("Glance %s (%s)\n", version, e1003.RendererVersion)
		return nil
	}
	defaults, err := config.DefaultPath()
	if err != nil {
		return err
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	configPath := flags.String("config", defaults, "owner-only configuration path")
	installDirectory := flags.String("install-directory", "", "installer target directory for configuration migration")
	legacyDirectory := flags.String("legacy-directory", "", "previous installation directory")
	output := flags.String("output", "var/preview.png", "PNG output for preview command")
	privacy := flags.Bool("privacy", false, "hide all task titles in demo")
	demo := flags.Bool("demo", false, "use synthetic data, no Codex access")
	page := flags.Int("page", 0, "zero-based page for preview command")
	variant := flags.String("variant", "limited", "preview quota: limited, full, missing-five, stale, unavailable, banner-sprint, banner-cruise, banner-snack, banner-sleep")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	switch os.Args[1] {
	case "migrate-config":
		if *installDirectory != "" {
			if err := config.MigrateInstallation(*configPath, *legacyDirectory, *installDirectory); err != nil {
				return err
			}
		}
		if err := config.MigratePreviewListen(*configPath); err != nil {
			return err
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		fmt.Println("Preview: http://" + cfg.PreviewListen)
		return nil
	case "init-config":
		if err := config.WriteInitial(*configPath); err != nil {
			return err
		}
		fmt.Println("Created owner-only configuration:", *configPath)
		return nil
	case "preview":
		if *page < 0 || *page > 1000 {
			return errors.New("invalid page")
		}
		r, err := nativeview.New("")
		if err != nil {
			return err
		}
		defer r.Close()
		d := companion.Demo(time.Date(2026, 10, 12, 10, 24, 0, 0, time.Local), *privacy)
		switch *variant {
		case "limited":
		case "full":
			d.Quota.Windows[0].UsedPercent = 0
			d.Quota.Windows[1].UsedPercent = 16
		case "missing-five":
			d.Quota.Windows = d.Quota.Windows[1:]
		case "stale":
			d.Quota.Windows = d.Quota.Windows[1:]
			d.QuotaStatus.State = model.SourceStale
		case "unavailable":
			d.Quota = nil
			d.QuotaStatus = model.SourceStatus{State: model.SourceUnavailable}
		case "banner-sprint", "banner-cruise", "banner-snack", "banner-sleep":
			d.Quota.ResetCredits = 0
			d.Quota.Windows[0].UsedPercent = map[string]float64{"banner-sprint": 20, "banner-cruise": 50, "banner-snack": 80, "banner-sleep": 100}[*variant]
		default:
			return errors.New("unknown preview variant")
		}
		f, err := r.RenderPage(d, *page)
		if err != nil {
			return err
		}
		if err = storage.AtomicWrite(*output, f.PNG); err != nil {
			return err
		}
		if err = storage.AtomicWrite(*output+".bin", f.Raw); err != nil {
			return err
		}
		fmt.Println("Wrote local preview", *output, "and reference pixels (not sent over USB); SHA256", f.SHA256)
		return nil
	case "serve":
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		// Single-instance lock also prevents competing writers and orphan observers.
		if err = os.MkdirAll(cfg.DataDirectory, 0700); err != nil {
			return err
		}
		lock, err := os.OpenFile(filepath.Join(cfg.DataDirectory, "companion.lock"), os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return err
		}
		defer lock.Close()
		if syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB) != nil {
			return errors.New("another companion instance already owns this data directory")
		}
		failures := make(chan error, 3)
		workers := 1
		var e *companion.Engine
		if *demo {
			e, err = companion.NewDemo(cfg)
		} else {
			e, err = companion.New(cfg)
		}
		if err != nil {
			return err
		}
		defer e.Close()
		store := e.Store
		var editor *server.NotesEditor
		if cfg.Transport == "usb" {
			editor, err = server.NewNotesEditor(e.Notes, func(ctx context.Context, notes []string, revision string) error {
				return e.SaveNotes(ctx, *configPath, notes, revision)
			})
			if err != nil {
				return err
			}
		}
		if !*demo {
			workers++
			go func() { failures <- e.Run(ctx) }()
		}
		deviceAddress := cfg.DeviceListen
		if cfg.Transport == "usb" {
			deviceAddress = ""
			if *demo {
				store.SetUSB(serialtransport.Status{State: "demo", Reason: "hardware_disabled"})
			} else {
				workers++
				go func() {
					failures <- serialtransport.Run(ctx, cfg.USBPort, cfg.USBDeviceID, e1003.Profile(), func() model.Dashboard { return store.Status().Dashboard }, store.SetUSB)
				}()
			}
		}
		go func() {
			failures <- server.Serve(ctx, deviceAddress, cfg.PreviewListen, cfg.DeviceToken, store, editor)
		}()
		fmt.Println("Preview: http://" + cfg.PreviewListen + " · transport: " + cfg.Transport)
		err = <-failures
		cancel()
		for i := 1; i < workers; i++ {
			<-failures
		}
		return err
	default:
		return errors.New("unknown command")
	}
}
