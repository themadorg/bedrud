package utils

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// SendSMTP sends an email via SMTP with support for SMTPS, STARTTLS, and plain modes.
func SendSMTP(addr string, auth smtp.Auth, from string, to []string, msg []byte, host string, tlsSkipVerify, smtpsMode bool) error {
	tlsCfg := &tls.Config{ServerName: host, InsecureSkipVerify: tlsSkipVerify}

	if smtpsMode {
		conn, err := tls.DialWithDialer(&net.Dialer{Timeout: 10 * time.Second}, "tcp", addr, tlsCfg)
		if err != nil {
			return fmt.Errorf("smtps dial: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			conn.Close()
			return fmt.Errorf("smtps new client: %w", err)
		}
		defer client.Close()
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtps auth: %w", err)
			}
		}
		return sendMailClient(client, from, to, msg)
	}

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp STARTTLS: %w", err)
		}
		if auth != nil {
			if err := client.Auth(auth); err != nil {
				return fmt.Errorf("smtp auth after STARTTLS: %w", err)
			}
		}
		return sendMailClient(client, from, to, msg)
	}

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth without TLS: %w (SMTP server does not support STARTTLS)", err)
		}
	}
	return sendMailClient(client, from, to, msg)
}

func sendMailClient(client *smtp.Client, from string, to []string, msg []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("rcpt %s: %w", addr, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return w.Close()
}

// BuildMessage builds a MIME multipart/alternative email message with HTML and plain text parts.
func BuildMessage(from, fromName, to, subject, bodyHTML, bodyPlain string) string {
	boundary := fmt.Sprintf("bedrud-boundary-%d", time.Now().UnixNano())
	msg := fmt.Sprintf("From: %s <%s>\r\n", fromName, from)
	msg += fmt.Sprintf("To: %s\r\n", to)
	msg += fmt.Sprintf("Subject: %s\r\n", subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	msg += "\r\n"
	msg += fmt.Sprintf("--%s\r\n", boundary)
	msg += "Content-Type: text/plain; charset=\"utf-8\"\r\n"
	msg += "\r\n"
	msg += bodyPlain + "\r\n"
	msg += fmt.Sprintf("\r\n--%s\r\n", boundary)
	msg += "Content-Type: text/html; charset=\"utf-8\"\r\n"
	msg += "Content-Transfer-Encoding: 8bit\r\n"
	msg += "\r\n"
	msg += bodyHTML + "\r\n"
	msg += fmt.Sprintf("\r\n--%s--\r\n", boundary)
	return msg
}
