package service

import (
	"bufio"
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
)

// A small SMTP server, enough to hold a conversation and hand back what it was
// told. Real enough to catch the things that actually break mail: a missing
// STARTTLS, a subject that went out as raw UTF-8, a body that was never encoded.
type fakeSMTP struct {
	addr     string
	received chan string
	offerTLS bool
	tlsConf  *tls.Config
}

func newFakeSMTP(t *testing.T, offerTLS bool) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })

	s := &fakeSMTP{
		addr:     ln.Addr().String(),
		received: make(chan string, 1),
		offerTLS: offerTLS,
		tlsConf:  selfSigned(t),
	}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go s.handle(conn)
		}
	}()
	return s
}

func (s *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	w := func(line string) { conn.Write([]byte(line + "\r\n")) }

	w("220 fake ESMTP")
	var body strings.Builder
	inData := false
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")

		if inData {
			if line == "." {
				inData = false
				select {
				case s.received <- body.String():
				default:
				}
				w("250 ok")
				continue
			}
			body.WriteString(line + "\n")
			continue
		}

		switch {
		case strings.HasPrefix(line, "EHLO"), strings.HasPrefix(line, "HELO"):
			if s.offerTLS {
				w("250-fake")
				w("250-STARTTLS")
				w("250 AUTH PLAIN")
			} else {
				w("250-fake")
				w("250 AUTH PLAIN")
			}
		case line == "STARTTLS":
			w("220 go ahead")
			tlsConn := tls.Server(conn, s.tlsConf)
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			r = bufio.NewReader(tlsConn)
			w = func(line string) { tlsConn.Write([]byte(line + "\r\n")) }
		case strings.HasPrefix(line, "AUTH"):
			w("235 ok")
		case strings.HasPrefix(line, "MAIL FROM"), strings.HasPrefix(line, "RCPT TO"):
			w("250 ok")
		case line == "DATA":
			inData = true
			w("354 go ahead")
		case line == "QUIT":
			w("221 bye")
			return
		default:
			w("250 ok")
		}
	}
}

func selfSigned(t *testing.T) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("cert: %v", err)
	}
	pair, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
	)
	if err != nil {
		t.Fatalf("keypair: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{pair}}
}

func splitHostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("split: %v", err)
	}
	n := 0
	for _, r := range port {
		n = n*10 + int(r-'0')
	}
	return host, n
}

func TestSMTPDeliverySendsTheToken(t *testing.T) {
	srv := newFakeSMTP(t, true)
	host, port := splitHostPort(t, srv.addr)

	d := SMTPDelivery{
		Host: host, Port: port,
		Username: "user", Password: "pass",
		From:   "noreply@example.jp",
		AppURL: "https://calendar.example.jp",
	}
	// The self-signed certificate would not verify; what is under test is the
	// conversation, not the trust chain.
	saved := tlsConfigFor
	tlsConfigFor = func(string) *tls.Config { return &tls.Config{InsecureSkipVerify: true} }
	t.Cleanup(func() { tlsConfigFor = saved })

	if err := d.SendPasswordReset("zav@example.jp", "abc123token"); err != nil {
		t.Fatalf("send: %v", err)
	}

	var raw string
	select {
	case raw = <-srv.received:
	case <-time.After(5 * time.Second):
		t.Fatal("the server never received a message")
	}

	if !strings.Contains(raw, "To: zav@example.jp") {
		t.Errorf("recipient missing:\n%s", raw)
	}
	if !strings.Contains(raw, "From: noreply@example.jp") {
		t.Errorf("sender missing:\n%s", raw)
	}
	// A Japanese subject has to be encoded; raw UTF-8 in a header is not legal
	// and shows up as mojibake in some clients.
	if strings.Contains(raw, "パスワード") {
		t.Errorf("subject went out as raw UTF-8:\n%s", raw)
	}
	if !strings.Contains(raw, "=?UTF-8?B?") {
		t.Errorf("subject is not encoded:\n%s", raw)
	}

	body := decodeBody(t, raw)
	if !strings.Contains(body, "abc123token") {
		t.Errorf("the token is not in the body:\n%s", body)
	}
	if !strings.Contains(body, "https://calendar.example.jp") {
		t.Errorf("the app link is missing:\n%s", body)
	}
	// Someone who did not ask for it should be told they can ignore it.
	if !strings.Contains(body, "心当たりがない") {
		t.Errorf("no note for an unexpected recipient:\n%s", body)
	}
}

func TestSMTPDeliverySendsTheTaskReminder(t *testing.T) {
	srv := newFakeSMTP(t, true)
	host, port := splitHostPort(t, srv.addr)

	d := SMTPDelivery{
		Host: host, Port: port,
		Username: "user", Password: "pass",
		From:   "noreply@example.jp",
		AppURL: "https://calendar.example.jp",
	}
	saved := tlsConfigFor
	tlsConfigFor = func(string) *tls.Config { return &tls.Config{InsecureSkipVerify: true} }
	t.Cleanup(func() { tlsConfigFor = saved })

	if err := d.SendTaskReminder("zav@example.jp", "レポート提出", "2026-08-15"); err != nil {
		t.Fatalf("send: %v", err)
	}

	var raw string
	select {
	case raw = <-srv.received:
	case <-time.After(5 * time.Second):
		t.Fatal("the server never received a message")
	}

	if !strings.Contains(raw, "To: zav@example.jp") {
		t.Errorf("recipient missing:\n%s", raw)
	}
	if !strings.Contains(raw, "=?UTF-8?B?") {
		t.Errorf("subject is not encoded:\n%s", raw)
	}

	body := decodeBody(t, raw)
	if !strings.Contains(body, "レポート提出") {
		t.Errorf("the task title is not in the body:\n%s", body)
	}
	if !strings.Contains(body, "2026-08-15") {
		t.Errorf("the due date is not in the body:\n%s", body)
	}
	if !strings.Contains(body, "https://calendar.example.jp") {
		t.Errorf("the app link is missing:\n%s", body)
	}
}

// A reset token in clear text hands over the account to anyone listening, so a
// server that will not upgrade gets nothing.
func TestSMTPDeliveryRefusesToSendWithoutTLS(t *testing.T) {
	srv := newFakeSMTP(t, false)
	host, port := splitHostPort(t, srv.addr)

	d := SMTPDelivery{Host: host, Port: port, From: "noreply@example.jp"}
	err := d.SendPasswordReset("zav@example.jp", "abc123token")
	if err == nil {
		t.Fatal("the mail went out over an unencrypted connection")
	}
	if !strings.Contains(err.Error(), "STARTTLS") {
		t.Fatalf("unexpected error: %v", err)
	}
	select {
	case raw := <-srv.received:
		t.Fatalf("a message was delivered anyway:\n%s", raw)
	default:
	}
}

func decodeBody(t *testing.T, raw string) string {
	t.Helper()
	_, encoded, ok := strings.Cut(raw, "\n\n")
	if !ok {
		t.Fatalf("no body separator in:\n%s", raw)
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\n", ""))
	if err != nil {
		t.Fatalf("body is not base64: %v\n%s", err, encoded)
	}
	return string(decoded)
}
