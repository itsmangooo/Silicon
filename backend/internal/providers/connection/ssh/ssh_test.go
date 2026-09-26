package sshconnection

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/itsmangooo/Silicon/backend/internal/providers/connection"
	"golang.org/x/crypto/ssh"
)

type testSSHServer struct {
	address     string
	fingerprint string
	privateKey  []byte
	commands    []string
	mu          sync.Mutex
	listener    net.Listener
}

func newTestSSHServer(t *testing.T) *testSSHServer {
	t.Helper()
	hostKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	hostSigner, err := ssh.NewSignerFromKey(hostKey)
	if err != nil {
		t.Fatal(err)
	}
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	clientSigner, err := ssh.NewSignerFromKey(clientKey)
	if err != nil {
		t.Fatal(err)
	}
	privateBlock, err := ssh.MarshalPrivateKey(clientKey, "")
	if err != nil {
		t.Fatal(err)
	}
	privatePEM := pem.EncodeToMemory(privateBlock)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &testSSHServer{address: listener.Addr().String(), fingerprint: ssh.FingerprintSHA256(hostSigner.PublicKey()), privateKey: privatePEM, listener: listener}
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if string(key.Marshal()) != string(clientSigner.PublicKey().Marshal()) {
			return nil, errors.New("denied")
		}
		return nil, nil
	}}
	config.AddHostKey(hostSigner)
	go server.serve(config)
	t.Cleanup(func() { _ = listener.Close() })
	return server
}

func (s *testSSHServer) serve(config *ssh.ServerConfig) {
	for {
		raw, err := s.listener.Accept()
		if err != nil {
			return
		}
		go func() {
			_, channels, requests, err := ssh.NewServerConn(raw, config)
			if err != nil {
				_ = raw.Close()
				return
			}
			go ssh.DiscardRequests(requests)
			for channel := range channels {
				if channel.ChannelType() != "session" {
					_ = channel.Reject(ssh.UnknownChannelType, "session only")
					continue
				}
				stream, requestChannel, err := channel.Accept()
				if err != nil {
					continue
				}
				go s.session(stream, requestChannel)
			}
		}()
	}
}

func (s *testSSHServer) session(stream ssh.Channel, requests <-chan *ssh.Request) {
	defer stream.Close()
	for request := range requests {
		if request.Type != "exec" {
			_ = request.Reply(false, nil)
			continue
		}
		var payload struct{ Command string }
		ssh.Unmarshal(request.Payload, &payload)
		s.mu.Lock()
		s.commands = append(s.commands, payload.Command)
		s.mu.Unlock()
		_ = request.Reply(true, nil)
		switch payload.Command {
		case "exec 'uname' '-s'":
			_, _ = io.WriteString(stream, "Linux\n")
		case "exec 'uname' '-m'":
			_, _ = io.WriteString(stream, "x86_64\n")
		case "exec 'docker' 'version' '--format' '{{.Server.Version}}'":
			_, _ = io.WriteString(stream, "28.3.0\n")
		case "exec 'docker' 'logs' '--timestamps' '--tail' '2' '--' 'container-id'":
			_, _ = io.WriteString(stream, "2026-01-01T00:00:00Z remote log\n")
		default:
			_, _ = io.Copy(io.Discard, stream)
		}
		_, _ = stream.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
		return
	}
}

func (s *testSSHServer) config() connection.Config {
	host, port, _ := net.SplitHostPort(s.address)
	parsedPort, _ := strconv.Atoi(port)
	return connection.Config{Type: "ssh", Host: host, Port: parsedPort, Username: "silicon", PrivateKey: append([]byte(nil), s.privateKey...)}
}

func TestHostKeyTrustAndActiveCheck(t *testing.T) {
	server := newTestSSHServer(t)
	provider := Provider{Timeout: 2 * time.Second}
	config := server.config()
	_, err := provider.Check(context.Background(), config)
	var keyError *connection.HostKeyError
	if !errors.As(err, &keyError) || keyError.Changed || keyError.Fingerprint != server.fingerprint {
		t.Fatalf("first check error=%#v", err)
	}
	config.HostKeyFingerprint = server.fingerprint
	status, err := provider.Check(context.Background(), config)
	if err != nil || !status.Reachable || !status.DockerAvailable || status.DockerVersion != "28.3.0" || status.OperatingSystem != "Linux" {
		t.Fatalf("status=%#v err=%v", status, err)
	}
	config.HostKeyFingerprint = "SHA256:unexpected"
	_, err = provider.Check(context.Background(), config)
	if !errors.As(err, &keyError) || !keyError.Changed || keyError.Fingerprint != server.fingerprint {
		t.Fatalf("changed key error=%#v", err)
	}
}

func TestAuthenticationFailureAndBoundedCommands(t *testing.T) {
	server := newTestSSHServer(t)
	provider := Provider{Timeout: 2 * time.Second}
	config := server.config()
	config.HostKeyFingerprint = server.fingerprint
	wrong, _ := rsa.GenerateKey(rand.Reader, 2048)
	block, _ := ssh.MarshalPrivateKey(wrong, "")
	config.PrivateKey = pem.EncodeToMemory(block)
	if _, err := provider.Check(context.Background(), config); !errors.Is(err, connection.ErrAuthenticationFailed) {
		t.Fatalf("authentication error=%v", err)
	}

	config = server.config()
	config.HostKeyFingerprint = server.fingerprint
	executor, err := provider.Executor(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = executor.Output(context.Background(), nil, "docker", "inspect", "value'; touch /tmp/unsafe; '"); err != nil {
		t.Fatal(err)
	}
	stream, err := executor.Start(context.Background(), "docker", "logs", "--timestamps", "--tail", "2", "--", "container-id")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(stream.Stdout)
	if err = stream.Wait(); err != nil || !strings.Contains(string(data), "remote log") {
		t.Fatalf("logs=%q err=%v", data, err)
	}
	server.mu.Lock()
	commands := strings.Join(server.commands, "\n")
	server.mu.Unlock()
	if !strings.Contains(commands, `'value'"'"'; touch /tmp/unsafe; '"'"''`) {
		t.Fatalf("argument was not safely quoted:\n%s", commands)
	}
}
