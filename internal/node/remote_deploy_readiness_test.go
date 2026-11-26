package node

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDeployAgentReadiness(t *testing.T) {
	for _, tc := range []struct {
		name       string
		readyAfter int32
		wantCalls  int32
	}{
		{name: "registered after a retry", readyAfter: 1, wantCalls: 2},
		{name: "registration never succeeds", readyAfter: 15, wantCalls: 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, binary := range []string{"sh", "curl", "grep"} {
				if _, err := exec.LookPath(binary); err != nil {
					t.Skipf("%s is required for the deployment script", binary)
				}
			}

			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health/ready" {
					http.NotFound(w, r)
					return
				}
				if requests.Add(1) <= tc.readyAfter {
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(`{"ready":false,"reasons":["not registered with panel"]}`))
					return
				}
				_, _ = w.Write([]byte(`{"ready":true}`))
			}))
			defer server.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", "-c", "sleep() { :; }\n"+verifyAgentReadinessScript)
			cmd.Env = append(cmd.Environ(), "HEALTH_PORT="+strconv.Itoa(server.Listener.Addr().(*net.TCPAddr).Port))
			output, err := cmd.CombinedOutput()
			if tc.readyAfter < tc.wantCalls {
				require.NoError(t, err, "%s", output)
				require.Contains(t, string(output), "Agent 已注册到面板")
			} else {
				require.ErrorAs(t, err, new(*exec.ExitError), "%s", output)
				require.Contains(t, string(output), "not registered with panel")
				require.NotContains(t, string(output), "Agent 已注册到面板")
			}
			require.Equal(t, tc.wantCalls, requests.Load())
		})
	}
}

func TestManualDeployScriptChecksReadinessBeforeSuccess(t *testing.T) {
	service := &RemoteDeployService{}
	script := service.GetDeployScript("https://panel.example.com", "node-token", 19443)
	require.Contains(t, script, "HEALTH_PORT=19443\n(\n"+verifyAgentReadinessScript+"\n)\n\necho")
}
