// Package sms is app.api.sms: where a one-time code leaves the server.
//
// Two implementations match Python. LogSender sends nothing and is what a host
// without a gateway runs. HTTPSender is one JSON POST with a bearer token.
// MOBILE_OTP_DEBUG_CODE is honoured only beside the log sender; a debug code
// on a host that can reach real phones refuses to start.
package sms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	GatewayURLEnv   = "MOBILE_SMS_GATEWAY_URL"
	GatewayTokenEnv = "MOBILE_SMS_GATEWAY_TOKEN"
	TemplateEnv     = "MOBILE_SMS_TEMPLATE"
	LogCodesEnv     = "MOBILE_OTP_LOG_CODES"
	DebugCodeEnv    = "MOBILE_OTP_DEBUG_CODE"
	DefaultTemplate = "Ma Ru Di cua ban: {code}. Ma het han sau 5 phut."
)

// debugCodeShape is _DEBUG_CODE_SHAPE: exactly six decimal digits.
var debugCodeShape = regexp.MustCompile("^[" + "0-9" + "]{6}$")

// Sender is SmsSender.
type Sender interface {
	SendOTP(canonicalPhone, code, challengeID string) error
}

// DeliveryError is SmsDeliveryError: the gateway did not accept the message.
// The text never carries a telephone number.
type DeliveryError struct {
	Message string
}

func (e *DeliveryError) Error() string { return e.Message }

// ConfigInvalid is OtpConfigInvalid: refuse to start rather than guess.
type ConfigInvalid struct {
	Message string
}

func (e *ConfigInvalid) Error() string { return e.Message }

// LogSender is LogSmsSender: no gateway. The number is never logged.
type LogSender struct {
	LogCodes bool
	Log      *slog.Logger
}

// SendOTP is LogSmsSender.send_otp.
func (s LogSender) SendOTP(canonicalPhone, code, challengeID string) error {
	_ = canonicalPhone
	log := s.Log
	if log == nil {
		log = slog.Default()
	}
	if s.LogCodes {
		log.Info("otp challenge issued", "challenge_id", challengeID, "code", code)
		return nil
	}
	log.Info("otp challenge issued", "challenge_id", challengeID)
	return nil
}

// HTTPSender is HttpJsonSmsSender: one JSON POST per code.
type HTTPSender struct {
	URL      string
	Token    string
	Template string
	Timeout  time.Duration
	Client   *http.Client
}

// SendOTP is HttpJsonSmsSender.send_otp. A transport failure names only the
// error type, because the exception text can carry the URL and the body.
func (s HTTPSender) SendOTP(canonicalPhone, code, challengeID string) error {
	template := s.Template
	if template == "" {
		template = DefaultTemplate
	}
	body, err := json.Marshal(map[string]string{
		"to":   canonicalPhone,
		"body": strings.ReplaceAll(template, "{code}", code),
	})
	if err != nil {
		return err
	}
	timeout := s.Timeout
	if timeout == 0 {
		timeout = 8 * time.Second
	}
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	} else if client.Timeout == 0 {
		clone := *client
		clone.Timeout = timeout
		client = &clone
	}
	req, err := http.NewRequest(http.MethodPost, s.URL, bytes.NewReader(body))
	if err != nil {
		return &DeliveryError{Message: fmt.Sprintf("%T", err)}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := client.Do(req)
	if err != nil {
		return &DeliveryError{Message: fmt.Sprintf("%T", err)}
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &DeliveryError{Message: fmt.Sprintf("gateway answered %d", resp.StatusCode)}
	}
	log := slog.Default()
	log.Info("otp challenge handed to gateway", "challenge_id", challengeID)
	return nil
}

// FromEnv is build_sms_sender plus resolve_otp_debug_code: the sender this
// host runs, and the debug code only when that sender is the log sender.
func FromEnv(getenv func(string) string) (Sender, *string, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	url := strings.TrimSpace(getenv(GatewayURLEnv))
	var sender Sender
	if url == "" {
		sender = LogSender{LogCodes: strings.TrimSpace(getenv(LogCodesEnv)) == "1"}
	} else {
		token := strings.TrimSpace(getenv(GatewayTokenEnv))
		if token == "" {
			return nil, nil, &ConfigInvalid{Message: GatewayURLEnv + " is set but " + GatewayTokenEnv + " is empty"}
		}
		template := strings.TrimSpace(getenv(TemplateEnv))
		if template == "" {
			template = DefaultTemplate
		}
		sender = HTTPSender{URL: url, Token: token, Template: template, Timeout: 8 * time.Second}
	}
	debug, err := resolveDebugCode(getenv, sender)
	if err != nil {
		return nil, nil, err
	}
	return sender, debug, nil
}

func resolveDebugCode(getenv func(string) string, sender Sender) (*string, error) {
	raw := strings.TrimSpace(getenv(DebugCodeEnv))
	if raw == "" {
		return nil, nil
	}
	if _, ok := sender.(LogSender); !ok {
		return nil, &ConfigInvalid{
			Message: DebugCodeEnv + " is set on a host with a real SMS gateway; refuse to start",
		}
	}
	if !debugCodeShape.MatchString(raw) {
		return nil, &ConfigInvalid{Message: DebugCodeEnv + " must be exactly six digits"}
	}
	return &raw, nil
}
