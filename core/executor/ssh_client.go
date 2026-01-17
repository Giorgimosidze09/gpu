package executor

import (
	"context"
	"fmt"
	"io"
	"log"

	// Phase 4: SSH client - uncomment when golang.org/x/crypto/ssh is added
	// "golang.org/x/crypto/ssh"
)

// SSHClient handles SSH connections to remote nodes
// Phase 4: Real SSH execution for training jobs
type SSHClient struct {
	// config *ssh.ClientConfig // Phase 4: Uncomment when golang.org/x/crypto/ssh is added
}

// NewSSHClient creates a new SSH client
func NewSSHClient(privateKey []byte, user string) (*SSHClient, error) {
	// Phase 4: Parse private key
	// TODO: Uncomment when golang.org/x/crypto/ssh is added:
	// signer, err := ssh.ParsePrivateKey(privateKey)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to parse private key: %w", err)
	// }
	// config := &ssh.ClientConfig{
	// 	User:            user,
	// 	Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
	// 	HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	// 	Timeout:         30 * time.Second,
	// }
	return &SSHClient{}, nil
}

// ExecuteCommand executes a command on a remote node via SSH
func (sc *SSHClient) ExecuteCommand(
	ctx context.Context,
	host string,
	command string,
) (string, error) {
	// Phase 4: Connect to remote host via SSH
	// TODO: Implement when golang.org/x/crypto/ssh is added
	log.Printf("Would execute command on %s: %s", host, command)
	return "", fmt.Errorf("SSH execution requires golang.org/x/crypto/ssh package")
}

// ExecuteCommandStream executes a command and streams output
func (sc *SSHClient) ExecuteCommandStream(
	ctx context.Context,
	host string,
	command string,
	outputWriter io.Writer,
) error {
	// Phase 4: Stream command output
	// TODO: Implement when golang.org/x/crypto/ssh is added
	return fmt.Errorf("SSH streaming requires golang.org/x/crypto/ssh package")
}

// CopyFile copies a file to remote node via SCP
func (sc *SSHClient) CopyFile(
	ctx context.Context,
	host string,
	localPath string,
	remotePath string,
) error {
	// Phase 4: Copy file using SCP
	log.Printf("Would copy file %s to %s:%s", localPath, host, remotePath)
	return fmt.Errorf("SCP not yet implemented")
}

// TestConnection tests SSH connection to a node
func (sc *SSHClient) TestConnection(ctx context.Context, host string) error {
	// Phase 4: Test SSH connectivity
	// TODO: Implement when golang.org/x/crypto/ssh is added
	return fmt.Errorf("SSH test requires golang.org/x/crypto/ssh package")
}
