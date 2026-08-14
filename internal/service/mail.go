package service

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// SMTPDelivery sends the password reset by email. It replaces LogDelivery,
// which only ever put the token in the server log — fine for finding out
// whether the flow worked, useless to the person who forgot their password.
//
// The connection is encrypted either way: port 465 is TLS from the first byte,
// anything else is upgraded with STARTTLS and the send is abandoned if the
// server will not upgrade. A reset token travelling in clear text is a handover
// of the account to whoever is listening.
type SMTPDelivery struct {
	Host     string // e.g. smtp.example.jp
	Port     int    // 587 (STARTTLS) or 465 (implicit TLS)
	Username string
	Password string
	From     string // the address the mail comes from
	AppURL   string // where the reset screen lives, for the link in the body
}

func (d SMTPDelivery) addr() string { return net.JoinHostPort(d.Host, itoa(d.Port)) }

func (d SMTPDelivery) SendPasswordReset(email, token string) error {
	subject := "他力本願カレンダー：パスワードの再設定"
	body := d.resetBody(token)
	return d.send(email, subject, body)
}

func (d SMTPDelivery) SendTaskReminder(email, title, dueAt string) error {
	subject := "他力本願カレンダー：明日が締切のタスクがあります"
	body := d.reminderBody(title, dueAt)
	return d.send(email, subject, body)
}

func (d SMTPDelivery) reminderBody(title, dueAt string) string {
	var b strings.Builder
	b.WriteString("締切が近いタスクがあります。\r\n\r\n")
	b.WriteString("    " + title + "\r\n")
	b.WriteString("    締切： " + dueAt + "\r\n\r\n")
	if d.AppURL != "" {
		b.WriteString("アプリはこちら： " + d.AppURL + "\r\n\r\n")
	}
	b.WriteString("すでに終わっている場合は、このメールは無視してください。\r\n")
	return b.String()
}

func (d SMTPDelivery) resetBody(token string) string {
	var b strings.Builder
	b.WriteString("パスワードの再設定を受け付けました。\r\n\r\n")
	b.WriteString("次のコードをアプリに入力してください。\r\n\r\n")
	b.WriteString("    " + token + "\r\n\r\n")
	if d.AppURL != "" {
		b.WriteString("アプリはこちら： " + d.AppURL + "\r\n\r\n")
	}
	b.WriteString("このコードは30分で使えなくなり、一度しか使えません。\r\n")
	// Someone who did not ask for this needs to know they can ignore it, and
	// that ignoring it changes nothing.
	b.WriteString("心当たりがない場合は、このメールを捨ててください。\r\n")
	b.WriteString("パスワードはそのままです。\r\n")
	return b.String()
}

func (d SMTPDelivery) send(to, subject, body string) error {
	msg := d.compose(to, subject, body)

	client, err := d.dial()
	if err != nil {
		return err
	}
	defer client.Close()

	if d.Username != "" {
		auth := smtp.PlainAuth("", d.Username, d.Password, d.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(d.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}
	return client.Quit()
}

// tlsConfigFor is a variable so the tests can point at a self-signed server.
// Production has no way to weaken it: there is no field for it on SMTPDelivery,
// because a "skip verification" toggle in configuration eventually gets set.
var tlsConfigFor = func(host string) *tls.Config {
	return &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
}

func (d SMTPDelivery) dial() (*smtp.Client, error) {
	tlsConfig := tlsConfigFor(d.Host)

	if d.Port == 465 {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 15 * time.Second}, "tcp", d.addr(), tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("smtp dial: %w", err)
		}
		return smtp.NewClient(conn, d.Host)
	}

	conn, err := net.DialTimeout("tcp", d.addr(), 15*time.Second)
	if err != nil {
		return nil, fmt.Errorf("smtp dial: %w", err)
	}
	client, err := smtp.NewClient(conn, d.Host)
	if err != nil {
		conn.Close()
		return nil, err
	}
	// No fallback to plaintext. If the server cannot upgrade, the mail does not
	// go — the token is worth an account.
	ok, _ := client.Extension("STARTTLS")
	if !ok {
		client.Close()
		return nil, fmt.Errorf("smtp: %s does not offer STARTTLS", d.Host)
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		client.Close()
		return nil, fmt.Errorf("smtp starttls: %w", err)
	}
	return client, nil
}

// compose builds the message. The subject is encoded because it is Japanese,
// and headers are ASCII-only on the wire.
func (d SMTPDelivery) compose(to, subject, body string) string {
	var b strings.Builder
	b.WriteString("From: " + d.From + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + encodeHeader(subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("\r\n")
	b.WriteString(base64Lines(body))
	return b.String()
}

// encodeHeader wraps a non-ASCII header value in RFC 2047 encoded-word form.
func encodeHeader(s string) string {
	return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
}

// base64Lines encodes the body and wraps it, since SMTP lines have a limit and
// some servers are strict about it.
func base64Lines(s string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(s))
	var b strings.Builder
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return b.String()
}
