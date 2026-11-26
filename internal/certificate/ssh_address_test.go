package certificate

import (
	"v/internal/database/repository"

	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
	"v/internal/logger"
)

// The listener closes before authentication: reaching the SSH handshake proves
// the real connection path parsed and dialed the host and port successfully.
func TestSSHConnectionAddressFamilies(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "::1"} {
		t.Run(host, func(t *testing.T) {
			listener, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(host)})
			if err != nil && host == "::1" {
				t.Skipf("IPv6 loopback unavailable: %v", err)
			}
			require.NoError(t, err)
			defer listener.Close()
			require.NoError(t, listener.SetDeadline(time.Now().Add(3*time.Second)))
			accepted := make(chan error, 1)
			go func() {
				conn, acceptErr := listener.Accept()
				if acceptErr == nil {
					conn.Close()
				}
				accepted <- acceptErr
			}()
			port := listener.Addr().(*net.TCPAddr).Port
			service := &Service{logger: logger.NewNopLogger()}
			err = service.deployViaSSH(&repository.Node{Address: host, SSHPort: port, SSHPassword: "test"}, "example.com", nil, nil)
			require.ErrorContains(t, err, "handshake")
			require.NoError(t, <-accepted)
		})
	}
}
