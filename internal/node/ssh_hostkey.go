package node

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var sshKnownHostsMu sync.Mutex

// SSHKnownHostsPath returns the trust file used for node SSH connections.
func SSHKnownHostsPath() (string, error) {
	if path := strings.TrimSpace(os.Getenv("VPANEL_SSH_KNOWN_HOSTS")); path != "" {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("无法定位 SSH known_hosts: %w", err)
	}
	return filepath.Join(home, ".ssh", "known_hosts"), nil
}

// SSHHostKeyCallback verifies node host keys against the trust file. A host
// seen for the first time is recorded (trust on first use); a host whose key
// later changes is rejected until an operator removes its line from the file.
// onFirstUse, when set, receives the fingerprint that was just recorded.
func SSHHostKeyCallback(onFirstUse func(host, fingerprint string)) (ssh.HostKeyCallback, string, error) {
	path, err := SSHKnownHostsPath()
	if err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, path, fmt.Errorf("无法创建 SSH 信任文件目录 %s: %w", filepath.Dir(path), err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, path, fmt.Errorf("无法写入 SSH 信任文件 %s: %w", path, err)
	}
	_ = file.Close()

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		sshKnownHostsMu.Lock()
		defer sshKnownHostsMu.Unlock()
		// Reload on every connection so entries added meanwhile are honoured.
		verify, err := knownhosts.New(path)
		if err != nil {
			return fmt.Errorf("无法读取 SSH 信任文件 %s: %w", path, err)
		}
		err = verify(hostname, remote, key)
		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) || len(keyErr.Want) != 0 {
			return err // nil, or a changed key
		}
		line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
		out, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			return fmt.Errorf("无法记录 SSH 主机公钥到 %s: %w", path, err)
		}
		_, writeErr := out.WriteString(line + "\n")
		if err := errors.Join(writeErr, out.Close()); err != nil {
			return fmt.Errorf("无法记录 SSH 主机公钥到 %s: %w", path, err)
		}
		if onFirstUse != nil {
			onFirstUse(hostname, ssh.FingerprintSHA256(key))
		}
		return nil
	}, path, nil
}
