// glance-device pairs protocol-v2 devices locally; legacy Wi-Fi configuration is explicit.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/zzzblxx/glance/internal/config"
	"github.com/zzzblxx/glance/internal/devices/e1003"
	"github.com/zzzblxx/glance/internal/storage"
	"github.com/zzzblxx/glance/internal/transport/serial"
	"go.bug.st/serial"
	"golang.org/x/term"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func exchange(port io.ReadWriter, request any, configure bool) error {
	data, err := json.Marshal(request)
	if err != nil {
		return errors.New("cannot encode configuration")
	}
	data = append(data, '\n')
	if n, err := port.Write(data); err != nil || n != len(data) {
		return errors.New("USB write failed")
	}
	deadline := time.Now().Add(45 * time.Second)
	var line []byte
	buf := make([]byte, 512)
	for time.Now().Before(deadline) {
		n, err := port.Read(buf)
		if err != nil {
			return errors.New("USB read failed")
		}
		for _, b := range buf[:n] {
			if b == '\n' {
				var response struct {
					OK         bool   `json:"ok"`
					Protocol   int    `json:"protocol"`
					Device     string `json:"device"`
					Configured bool   `json:"configured"`
				}
				if json.Unmarshal(line, &response) == nil {
					if !response.OK {
						return errors.New("device rejected configuration")
					}
					if configure && response.Configured {
						return nil
					}
					if !configure && response.Protocol == 1 && response.Device == "e1003" {
						return nil
					}
				}
				line = nil
			} else {
				line = append(line, b)
				if len(line) > 4096 {
					return errors.New("unexpected USB response")
				}
			}
		}
	}
	return errors.New("device did not acknowledge within 45 seconds")
}
func run() error {
	defaultPath, _ := config.DefaultPath()
	configPath := flag.String("config", defaultPath, "companion configuration (token read locally)")
	portName := flag.String("port", "", "explicit E1003 USB serial device, e.g. /dev/cu.wchusbserial...")
	legacy := flag.Bool("legacy-wifi", false, "configure protocol-v1 Wi-Fi firmware instead of pairing USB")
	ssid := flag.String("ssid", "", "Wi-Fi SSID")
	serverURL := flag.String("server", "", "Mac LAN address, e.g. http://192.168.1.10:8787")
	list := flag.Bool("list", false, "list USB serial ports")
	flag.Parse()
	if *list {
		ports, err := serial.GetPortsList()
		if err != nil {
			return err
		}
		for _, p := range ports {
			fmt.Println(p)
		}
		return nil
	}
	if !*legacy {
		if *portName == "" {
			return errors.New("--port is required; use --list to list ports")
		}
		if *ssid != "" || *serverURL != "" {
			return errors.New("Wi-Fi flags require --legacy-wifi")
		}
		cfg, err := config.Load(*configPath)
		if err != nil {
			return err
		}
		port, client, err := serialtransport.Open(*portName, e1003.Profile())
		if err != nil {
			return err
		}
		defer port.Close()
		// Bound blocked writes too; no dashboard or credentials are sent while pairing.
		timer := time.AfterFunc(45*time.Second, func() { _ = port.Close() })
		defer timer.Stop()
		hello, err := client.Hello()
		if err != nil {
			return errors.New("no protocol-v2 E1003 handshake; stop the companion and check firmware/cable/port")
		}
		cfg.Transport, cfg.USBPort, cfg.USBDeviceID = "usb", *portName, hello.ID
		if err = cfg.Validate(); err != nil {
			return err
		}
		encoded, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		if err = storage.AtomicWrite(*configPath, append(encoded, '\n')); err != nil {
			return err
		}
		fmt.Println("E1003 paired locally. Restart the companion; no Wi-Fi configuration or dashboard was sent.")
		return nil
	}
	if *portName == "" || *ssid == "" || len(*ssid) > 32 {
		return errors.New("--port and --ssid (up to 32 bytes) are required")
	}
	u, err := url.Parse(*serverURL)
	if err != nil || u.Scheme != "http" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("--server must be an HTTP origin without credentials or path")
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("run in a terminal to enter the Wi-Fi password securely")
	}
	fmt.Fprint(os.Stderr, "Wi-Fi password (hidden, empty for open network): ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return errors.New("cannot read Wi-Fi password")
	}
	defer clear(password)
	if len(password) > 63 {
		return errors.New("Wi-Fi password exceeds 63 bytes")
	}
	port, err := serial.Open(*portName, &serial.Mode{BaudRate: 115200})
	if err != nil {
		return errors.New("cannot open USB serial device")
	}
	defer port.Close()
	if err = port.SetReadTimeout(250 * time.Millisecond); err != nil {
		return err
	}
	_ = port.SetDTR(true)
	if err = exchange(port, map[string]string{"op": "ping"}, false); err != nil {
		return err
	}
	if err = exchange(port, map[string]string{"op": "configure", "ssid": *ssid, "password": string(password), "base_url": strings.TrimSuffix(*serverURL, "/"), "token": cfg.DeviceToken}, true); err != nil {
		return err
	}
	fmt.Println("Device configuration saved. Firmware unchanged; Wi-Fi reconnecting.")
	return nil
}
