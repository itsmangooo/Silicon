package smtp

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	mailprovider "github.com/itsmangooo/Silicon/backend/internal/providers/mail"
)

func TestEncodeMessageUsesMultipartAndRejectsHeaderInjection(t *testing.T) {
	message := mailprovider.Message{FromName: "Silicon", From: "silicon@example.com", To: "user@example.com", ReplyTo: "support@example.com", Subject: "Recovery", Text: "plain body", HTML: "<p>html body</p>"}
	encoded, err := encodeMessage(message)
	if err != nil {
		t.Fatal(err)
	}
	value := string(encoded)
	for _, expected := range []string{"multipart/alternative", "plain body", "<p>html body</p>", "Reply-To: <support@example.com>"} {
		if !strings.Contains(value, expected) {
			t.Fatalf("encoded SMTP message is missing %q: %s", expected, value)
		}
	}
	message.Subject = "unsafe\r\nBcc: attacker@example.com"
	if err := mailprovider.ValidateMessage(message); err == nil {
		t.Fatal("message header injection was accepted")
	}
}

func TestSMTPRequiresEncryptedTransportForAuthentication(t *testing.T) {
	provider := Provider{Host: "mail.example.com", Port: 25, Username: "silicon", Password: []byte("secret"), Encryption: "none"}
	if err := provider.authenticate(nil); err == nil || err.Error() != "SMTP authentication requires STARTTLS or implicit TLS" {
		t.Fatalf("unexpected SMTP authentication result: %v", err)
	}
}

func TestSMTPTLSModesAndAuthentication(t *testing.T) {
	for _, mode := range []string{"starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			server := newSMTPServer(t, mode)
			defer server.close()
			provider := Provider{
				Host: "localhost", Port: server.port, Username: "silicon", Password: []byte("smtp-secret"), Encryption: mode,
				Timeout: 2 * time.Second, TLSConfig: &tls.Config{RootCAs: server.roots, ServerName: "localhost", MinVersion: tls.VersionTLS12},
			}
			message := mailprovider.Message{From: "silicon@example.com", To: "operator@example.com", Subject: "Test", Text: "plain", HTML: "<p>html</p>"}
			if err := provider.Send(context.Background(), message); err != nil {
				t.Fatal(err)
			}
			result := <-server.result
			if result.auth != "\x00silicon\x00smtp-secret" {
				t.Fatalf("unexpected SMTP authentication payload: %q", result.auth)
			}
			if !strings.Contains(result.message, "Subject: Test") || !strings.Contains(result.message, "plain") {
				t.Fatalf("message was not delivered over %s: %q", mode, result.message)
			}
		})
	}
}

func TestSMTPConnectionTimeoutIsBounded(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			defer connection.Close()
			time.Sleep(250 * time.Millisecond)
		}
	}()
	port := listener.Addr().(*net.TCPAddr).Port
	started := time.Now()
	err = (Provider{Host: "localhost", Port: port, Encryption: "none", Timeout: 50 * time.Millisecond}).Test(context.Background())
	if err == nil || err.Error() != "SMTP server greeting is invalid" {
		t.Fatalf("unexpected timeout error: %v", err)
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("SMTP greeting timeout was not bounded")
	}
}

type smtpTestResult struct {
	auth    string
	message string
}

type smtpTestServer struct {
	listener net.Listener
	port     int
	roots    *x509.CertPool
	result   chan smtpTestResult
}

func newSMTPServer(t *testing.T, mode string) smtpTestServer {
	t.Helper()
	certificate, roots := testCertificate(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := smtpTestServer{listener: listener, port: listener.Addr().(*net.TCPAddr).Port, roots: roots, result: make(chan smtpTestResult, 1)}
	go server.serve(mode, certificate)
	return server
}

func (s smtpTestServer) close() { _ = s.listener.Close() }

func (s smtpTestServer) serve(mode string, certificate tls.Certificate) {
	connection, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer connection.Close()
	secure := mode == "tls"
	if secure {
		connection = tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
	}
	reader := bufio.NewReader(connection)
	writer := bufio.NewWriter(connection)
	writeSMTP(writer, "220 localhost ESMTP ready\r\n")
	result := smtpTestResult{}
	for {
		line, readErr := reader.ReadString('\n')
		if readErr != nil {
			return
		}
		command := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(command, "EHLO"):
			if !secure && mode == "starttls" {
				writeSMTP(writer, "250-localhost\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n")
			} else {
				writeSMTP(writer, "250-localhost\r\n250 AUTH PLAIN\r\n")
			}
		case command == "STARTTLS":
			writeSMTP(writer, "220 Ready to start TLS\r\n")
			connection = tls.Server(connection, &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12})
			if err = connection.(*tls.Conn).Handshake(); err != nil {
				return
			}
			secure = true
			reader = bufio.NewReader(connection)
			writer = bufio.NewWriter(connection)
		case strings.HasPrefix(command, "AUTH PLAIN "):
			decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimPrefix(command, "AUTH PLAIN "))
			if decodeErr != nil {
				return
			}
			result.auth = string(decoded)
			writeSMTP(writer, "235 Authentication successful\r\n")
		case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
			writeSMTP(writer, "250 OK\r\n")
		case command == "DATA":
			writeSMTP(writer, "354 End data with <CR><LF>.<CR><LF>\r\n")
			var message strings.Builder
			for {
				dataLine, dataErr := reader.ReadString('\n')
				if dataErr != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				message.WriteString(dataLine)
			}
			result.message = message.String()
			writeSMTP(writer, "250 Queued\r\n")
		case command == "QUIT":
			writeSMTP(writer, "221 Bye\r\n")
			s.result <- result
			return
		default:
			writeSMTP(writer, "502 Command not implemented\r\n")
		}
	}
}

func writeSMTP(writer *bufio.Writer, value string) {
	_, _ = writer.WriteString(value)
	_ = writer.Flush()
}

func testCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "localhost"}, DNSNames: []string{"localhost"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment | x509.KeyUsageCertSign,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	certificate, err := tls.X509KeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(certificatePEM)
	return certificate, roots
}
