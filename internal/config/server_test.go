package config_test

import (
	"strings"
	"testing"

	"github.com/fmartingr/taskqueue/internal/tqtest"

	"github.com/fmartingr/taskqueue/internal/config"
)

func TestServerBindIsAbsentWithoutAConfigFile(t *testing.T) {
	cfg, err := config.FindConfig(tqtest.RootWithoutMarker(t))
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if cfg != nil {
		t.Fatalf("config.FindConfig() = %+v, want nil", cfg)
	}
	// A nil config answers both, so callers never check before asking.
	if host := cfg.ServerHost(); host != "" {
		t.Errorf("ServerHost() = %q, want empty", host)
	}
	if port, ok := cfg.ServerPort(); ok {
		t.Errorf("ServerPort() = %d, %v, want it absent", port, ok)
	}
}

func TestServerBindIsAbsentWhenTheKeyIs(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if host := cfg.ServerHost(); host != "" {
		t.Errorf("ServerHost() = %q, want empty", host)
	}
	if _, ok := cfg.ServerPort(); ok {
		t.Error("ServerPort() reported a port for a config that pins none")
	}
}

func TestServerBindReadsBothKeys(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  host: 0.0.0.0\n  port: 7412\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	if host := cfg.ServerHost(); host != "0.0.0.0" {
		t.Errorf("ServerHost() = %q, want 0.0.0.0", host)
	}
	port, ok := cfg.ServerPort()
	if !ok || port != 7412 {
		t.Errorf("ServerPort() = %d, %v, want 7412, true", port, ok)
	}
}

// Zero is a real answer — it asks the OS to pick — so it must not read as
// "no port pinned". This is why the field is a pointer.
func TestServerPortZeroIsPinnedNotAbsent(t *testing.T) {
	root := tqtest.Root(t)
	tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  port: 0\n")

	cfg, err := config.FindConfig(root)
	if err != nil {
		t.Fatalf("config.FindConfig: %v", err)
	}
	port, ok := cfg.ServerPort()
	if !ok {
		t.Fatal("ServerPort() reported port 0 as absent; the OS-picks-a-port case is lost")
	}
	if port != 0 {
		t.Errorf("ServerPort() = %d, want 0", port)
	}
}

// A nonsense port must not reach the listener, and the message has to name the
// file so the reader knows which project to fix.
func TestServerPortErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"above the range", "server:\n  port: 65536\n", "out of range"},
		{"negative", "server:\n  port: -1\n", "out of range"},
		{"not a number", "server:\n  port: seven\n", "cannot unmarshal"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := tqtest.Root(t)
			path := tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\n"+tc.body)

			_, err := config.FindConfig(root)
			if err == nil {
				t.Fatal("config.FindConfig() = nil, want an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error %q should name %s", err, path)
			}
		})
	}
}

// The board is served without authentication, so a host that reaches off this
// machine is worth saying out loud — especially from a committed file.
func TestExposedHost(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "localhost", "::1", "127.0.0.53"} {
		if config.ExposedHost(host) {
			t.Errorf("ExposedHost(%q) = true, want false: it is loopback", host)
		}
	}
	for _, host := range []string{"0.0.0.0", "", "::", "192.168.1.10", "example.internal"} {
		if !config.ExposedHost(host) {
			t.Errorf("ExposedHost(%q) = false, want true: it reaches off this machine", host)
		}
	}
}

// The README promises an invalid value is refused when the config loads. That
// has to cover the host too: a bracketed IPv6 address is the spelling people
// reach for, and left alone it reaches net.Listen as `[[::1]]:7331` — an error
// naming neither the file nor the key — while ExposedHost, unable to parse it,
// warns that a loopback address is reachable from the network.
func TestServerHostErrors(t *testing.T) {
	for _, tc := range []struct{ name, host, wantErr string }{
		{"bracketed IPv6", "[::1]", "write it unbracketed, as ::1"},
		{"an address with a port on it", "127.0.0.1:7331", "neither an IP address nor a hostname"},
		{"a URL", "http://localhost", "neither an IP address nor a hostname"},
		{"a space", "local host", "neither an IP address nor a hostname"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tqtest.Root(t)
			path := tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  host: \""+tc.host+"\"\n")

			_, err := config.FindConfig(root)
			if err == nil {
				t.Fatalf("config.FindConfig() = nil, want %q refused", tc.host)
			}
			if !strings.Contains(err.Error(), tc.wantErr) || !strings.Contains(err.Error(), path) {
				t.Errorf("error = %q, want it to contain %q and name %s", err, tc.wantErr, path)
			}
		})
	}
}

func TestServerHostAccepts(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "0.0.0.0", "::1", "::", "localhost", "board.internal", "tq-board"} {
		root := tqtest.Root(t)
		tqtest.WriteConfig(t, root, "version: 1\npath: .tasks\nserver:\n  host: \""+host+"\"\n")
		if _, err := config.FindConfig(root); err != nil {
			t.Errorf("config.FindConfig() with host %q = %v, want it accepted", host, err)
		}
	}
}
