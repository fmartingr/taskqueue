package cli

import (
	"bytes"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/fmartingr/taskqueue/internal/tqtest"
)

// The four layers of the bind address, and the order they beat each other in.
// The flag layer sits above all of these and is covered separately, since it
// only exists once the flag set has parsed.
func TestServeDefaultsPrecedence(t *testing.T) {
	const pinned = "version: 1\npath: .tasks\nserver:\n  host: 10.0.0.5\n  port: 7412\n"

	tests := []struct {
		name     string
		config   string
		envHost  string
		envPort  string
		wantHost string
		wantPort string
	}{
		{
			name:     "nothing set falls back to the built-in address",
			wantHost: defaultHost,
			wantPort: defaultPort,
		},
		{
			name:     "the project's config beats the built-in",
			config:   pinned,
			wantHost: "10.0.0.5",
			wantPort: "7412",
		},
		{
			name:     "the environment beats the config",
			config:   pinned,
			envHost:  "192.168.0.9",
			envPort:  "9001",
			wantHost: "192.168.0.9",
			wantPort: "9001",
		},
		{
			name:     "the environment also beats the built-in on its own",
			envPort:  "9001",
			wantHost: defaultHost,
			wantPort: "9001",
		},
		{
			name:     "each key stands alone: a pinned port leaves the host on the default",
			config:   "version: 1\npath: .tasks\nserver:\n  port: 7412\n",
			wantHost: defaultHost,
			wantPort: "7412",
		},
		{
			name:     "port 0 is pinned, not absent",
			config:   "version: 1\npath: .tasks\nserver:\n  port: 0\n",
			wantHost: defaultHost,
			wantPort: "0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc := tc
			// A case with no config needs a root that genuinely has no
			// marker, at it or anywhere above it.
			root := tqtest.RootWithoutMarker(t)
			if tc.config != "" {
				tqtest.WriteConfig(t, root, tc.config)
			}
			t.Setenv("TQ_HOST", tc.envHost)
			t.Setenv("TQ_PORT", tc.envPort)

			c := &cli{dir: root, version: testVersion}
			host, port := c.serveDefaults()
			if host != tc.wantHost {
				t.Errorf("host = %q, want %q", host, tc.wantHost)
			}
			if port != tc.wantPort {
				t.Errorf("port = %q, want %q", port, tc.wantPort)
			}
		})
	}
}

// The interesting one: a flag still beats an address the project committed.
func TestServeFlagBeatsTheConfig(t *testing.T) {
	tc := newBareCLI(t)
	tqtest.WriteConfig(t, tc.root, "version: 1\npath: .tasks\nserver:\n  host: 10.0.0.5\n  port: 7412\n")
	tc.mustRun("init")
	t.Setenv("TQ_HOST", "")
	t.Setenv("TQ_PORT", "")

	// Port 0 asks the OS to pick, so this never has to guess a free one — and
	// the banner prints what it got, which is how the assertion reads it back.
	done := make(chan int, 1)
	go func() { done <- tc.run("serve", "--host", "127.0.0.1", "--port", "0") }()
	line := awaitBanner(t, tc)

	if !strings.Contains(line, "http://127.0.0.1:") {
		t.Errorf("banner = %q, want the flag's host rather than the config's 10.0.0.5", line)
	}
	if strings.Contains(line, ":7412") {
		t.Errorf("banner = %q, want the flag's port rather than the config's 7412", line)
	}
	stopServe(t, done)
}

// A committed host that reaches off the machine is said out loud, because the
// board has no authentication in front of it.
//
// This drives the warning directly rather than through `tq serve --host
// 0.0.0.0`. Binding a real listener to every interface is a side effect a unit
// test has no business having: on a developer's machine it raises a firewall
// prompt, and on CI or a shared host it puts an unauthenticated task board on
// the network for as long as the test runs. What the CLI owns here is the
// message; whether an address is exposed is config.ExposedHost's decision, and
// it has its own tests. TestServeStaysQuietOnLoopback covers the other half
// through a real serve, which it can do because loopback is safe to bind.
func TestServeExposureWarning(t *testing.T) {
	for _, host := range []string{"0.0.0.0", "192.168.1.10", "::"} {
		var out bytes.Buffer
		warnIfExposed(&out, host)

		warning := out.String()
		if !strings.Contains(warning, host) || !strings.Contains(warning, "no authentication") {
			t.Errorf("warning for %q = %q, want it to name the host and the reason", host, warning)
		}
	}

	for _, host := range []string{"127.0.0.1", "localhost", "::1"} {
		var out bytes.Buffer
		warnIfExposed(&out, host)
		if out.Len() != 0 {
			t.Errorf("warning for loopback %q = %q, want silence", host, out.String())
		}
	}
}

func TestServeStaysQuietOnLoopback(t *testing.T) {
	tc := newTestCLI(t)
	t.Setenv("TQ_HOST", "")
	t.Setenv("TQ_PORT", "")

	done := make(chan int, 1)
	go func() { done <- tc.run("serve", "--port", "0") }()
	awaitBanner(t, tc)

	if warning := tc.stderr.String(); strings.Contains(warning, "no authentication") {
		t.Errorf("stderr = %q, want no exposure warning for the loopback default", warning)
	}
	stopServe(t, done)
}

// awaitBanner waits for the line `tq serve` prints once it is listening.
func awaitBanner(t *testing.T, tc *testCLI) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if s := tc.stdout.String(); strings.Contains(s, "http://") {
			return strings.SplitN(s, "\n", 2)[0]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no banner within the deadline; stderr = %q", tc.stderr)
	return ""
}

func stopServe(t *testing.T, done <-chan int) {
	t.Helper()
	if p, err := os.FindProcess(os.Getpid()); err == nil {
		_ = p.Signal(syscall.SIGTERM)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not shut down")
	}
}
