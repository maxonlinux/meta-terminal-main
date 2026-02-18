package otp

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (s *Service) sendEmail(email, code string) error {
	if email == "" {
		return ErrEmailConfig
	}
	if s.cfg.SmtpHost == "" || s.cfg.SmtpUser == "" || s.cfg.SmtpPassword == "" || s.cfg.SmtpFrom == "" {
		return ErrEmailConfig
	}

	from := s.cfg.SmtpFrom
	subject := "Your OTP Code"
	text := fmt.Sprintf(
		"Hello,\n\nYour one-time password (OTP) is: %s\n\nPlease never share this code with others.\n\nThank you,\n%s Team",
		code,
		s.cfg.SiteName,
	)
	html := fmt.Sprintf(`
<div style="font-family: Arial, sans-serif; line-height: 1.5; color: #333;">
  <h1 style="color:rgb(55, 78, 253);">Your OTP Code</h1>
  <p>Hello,</p>
  <p>Your one-time password (OTP) is:</p>
  <p style="font-size: 24px; font-weight: bold; color: rgb(55, 78, 253);">%s</p>
  <p>Please never share this code with others.</p>
  <hr>
  <p>
    Thank you,
    <br>
    %s Team
  </p>
</div>`,
		code,
		s.cfg.SiteName,
	)

	msg := buildMultipartEmail(from, email, subject, text, html)
	addr := net.JoinHostPort(s.cfg.SmtpHost, strconv.Itoa(s.cfg.SmtpPort))

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return ErrSendEmail
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.cfg.SmtpHost)
	if err != nil {
		return ErrSendEmail
	}
	defer func() {
		_ = client.Quit()
	}()

	if ok, _ := client.Extension("STARTTLS"); ok {
		_ = client.StartTLS(&tls.Config{
			ServerName:         s.cfg.SmtpHost,
			InsecureSkipVerify: s.cfg.SmtpSkipVerify,
		})
	}

	if s.cfg.SmtpUser != "" && s.cfg.SmtpPassword != "" {
		auth := smtp.PlainAuth("", s.cfg.SmtpUser, s.cfg.SmtpPassword, s.cfg.SmtpHost)
		if err := client.Auth(auth); err != nil {
			return ErrSendEmail
		}
	}

	if err := client.Mail(from); err != nil {
		return ErrSendEmail
	}
	if err := client.Rcpt(email); err != nil {
		return ErrSendEmail
	}

	w, err := client.Data()
	if err != nil {
		return ErrSendEmail
	}
	if _, err := io.Copy(w, bytes.NewBufferString(msg)); err != nil {
		_ = w.Close()
		return ErrSendEmail
	}
	_ = w.Close()

	return nil
}

func (s *Service) sendSms(phone, code string) error {
	if s.cfg.SmsAuthToken == "" {
		return ErrSmsConfig
	}

	values := url.Values{}
	values.Set("to", phone)
	values.Set("from", s.cfg.SiteName)
	values.Set("text", fmt.Sprintf("OTP Code: %s. Do not share it with others.", code))

	req, err := http.NewRequest(
		http.MethodPost,
		"https://gateway.seven.io/api/sms",
		strings.NewReader(values.Encode()),
	)
	if err != nil {
		return ErrSendSms
	}
	req.Header.Set("X-Api-Key", s.cfg.SmsAuthToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ErrSendSms
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ErrSendSms
	}

	return nil
}

func buildMultipartEmail(from, to, subject, text, html string) string {
	boundary := "mixed-otp-boundary"

	var b strings.Builder
	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", to))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n", boundary))
	b.WriteString("\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	b.WriteString(text + "\r\n")

	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	b.WriteString(html + "\r\n")

	b.WriteString("--" + boundary + "--\r\n")

	return b.String()
}
