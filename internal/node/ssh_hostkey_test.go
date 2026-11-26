package node

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func testHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	public, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	key, err := ssh.NewPublicKey(public)
	require.NoError(t, err)
	return key
}

func TestSSHHostKeyCallbackTrustsFirstUseAndRejectsChangedKey(t *testing.T) {
	t.Setenv("VPANEL_SSH_KNOWN_HOSTS", filepath.Join(t.TempDir(), "configs", "ssh_known_hosts"))
	recorded := 0
	verify, _, err := SSHHostKeyCallback(func(string, string) { recorded++ })
	require.NoError(t, err)

	addr := &net.TCPAddr{IP: net.ParseIP("203.0.113.7"), Port: 22}
	first := testHostKey(t)
	require.NoError(t, verify("203.0.113.7:22", addr, first))
	require.NoError(t, verify("203.0.113.7:22", addr, first))
	require.Equal(t, 1, recorded)
	require.Error(t, verify("203.0.113.7:22", addr, testHostKey(t)))

	// Another host is independent of the first.
	other := &net.TCPAddr{IP: net.ParseIP("2001:db8::7"), Port: 2222}
	require.NoError(t, verify("[2001:db8::7]:2222", other, testHostKey(t)))
	require.Equal(t, 2, recorded)
}
