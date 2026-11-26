// Package mail — email yuborish (SMTP). Kirish kodlari uchun.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, m Message) error
}

var ErrNotConfigured = errors.New("email yuborish sozlanmagan (SMTP_HOST)")

// SMTP — 465 (implicit TLS) yoki 587 (STARTTLS) portlari.
type SMTP struct {
	Host, User, Password, From string
	Port                       int
}

func (s SMTP) Send(ctx context.Context, m Message) error {
	if s.Host == "" {
		return ErrNotConfigured
	}
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("MAIL_FROM: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(s.Host, fmt.Sprint(s.Port))
	d := net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if s.Port == 465 {
		conn, err = tls.DialWithDialer(&d, "tcp", addr, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("SMTP server STARTTLS ni qo'llamaydi — parol ochiq yuborilmaydi")
		}
		if err := c.StartTLS(&tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return err
		}
	}
	if s.User != "" {
		if err := c.Auth(smtp.PlainAuth("", s.User, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(build(from, to, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

func build(from, to *mail.Address, m Message) []byte {
	boundary := fmt.Sprintf("yoldosh-%d", time.Now().UnixNano())
	var b strings.Builder
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", from.String())
	h("To", to.String())
	h("Subject", mime.QEncoding.Encode("utf-8", m.Subject))
	h("Date", time.Now().Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	h("Message-ID", fmt.Sprintf("<%s@%s>", boundary, domainOf(from.Address)))
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n")
	part := func(ct, body string) {
		fmt.Fprintf(&b, "--%s\r\nContent-Type: %s; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n%s\r\n", boundary, ct, body)
	}
	part("text/plain", m.Text)
	if m.HTML != "" {
		part("text/html", m.HTML)
	}
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String())
}

func domainOf(addr string) string {
	if i := strings.LastIndex(addr, "@"); i >= 0 {
		return addr[i+1:]
	}
	return "localhost"
}

// DevLog — faqat lokal ishlab chiqish uchun: xat yuborilmaydi, mazmuni logga yoziladi.
// Productionda ishlatilmasin (kodlar logga tushadi).
type DevLog struct{}

func (DevLog) Send(_ context.Context, m Message) error {
	slog.Warn("DEV MAIL (yuborilmadi)", "to", m.To, "subject", m.Subject, "text", m.Text)
	return nil
}

// Capture — testlar uchun: oxirgi xatlarni saqlaydi.
type Capture struct{ Sent []Message }

func (c *Capture) Send(_ context.Context, m Message) error {
	c.Sent = append(c.Sent, m)
	return nil
}
