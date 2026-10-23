package server

import (
	"net"
	"net/http"
	"testing"
	"time"

	"bedrud/config"

	"github.com/gofiber/fiber/v2"
)

// listenTimeout bounds how long the test waits for the listener to come up.
const listenTimeout = 5 * time.Second

func TestServeListeners(t *testing.T) {
	t.Run("should accept IPv4 and IPv6 loopback in plain-HTTP mode", func(t *testing.T) {
		probe, err := net.Listen("tcp6", "[::1]:0")
		if err != nil {
			t.Skipf("host has no IPv6 loopback: %v", err)
		}
		probe.Close()

		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		app.Get("/api/health", func(context *fiber.Ctx) error {
			return context.SendString("ok")
		})
		portChannel := make(chan string, 1)
		app.Hooks().OnListen(func(listenData fiber.ListenData) error {
			portChannel <- listenData.Port
			return nil
		})

		cfg := &config.Config{Server: config.ServerConfig{Port: "0"}}
		serveErr := make(chan error, 1)
		go func() { serveErr <- serveListeners(app, cfg) }()
		t.Cleanup(func() { _ = app.Shutdown() })

		var port string
		select {
		case port = <-portChannel:
		case err := <-serveErr:
			t.Fatalf("serveListeners returned before listening: %v", err)
		case <-time.After(listenTimeout):
			t.Fatal("listener did not start")
		}

		client := &http.Client{Timeout: listenTimeout}
		for _, host := range []string{"127.0.0.1", "[::1]"} {
			response, err := client.Get("http://" + host + ":" + port + "/api/health")
			if err != nil {
				t.Errorf("GET via %s: %v", host, err)
				continue
			}
			response.Body.Close()
			if response.StatusCode != http.StatusOK {
				t.Errorf("GET via %s: status %d, want %d", host, response.StatusCode, http.StatusOK)
			}
		}
	})
}
