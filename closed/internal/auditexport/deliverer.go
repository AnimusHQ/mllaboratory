package auditexport

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/animus-labs/animus-go/closed/internal/domain"
)

func EncodeEvent(event domain.AuditEvent) ([]byte, error) {
	payload := exportEventFromDomain(event)
	return json.Marshal(payload)
}

func DecodeSinkConfig(sink Sink) (SinkConfig, error) {
	if len(sink.Config) == 0 {
		return SinkConfig{}, nil
	}
	var cfg SinkConfig
	if err := json.Unmarshal(sink.Config, &cfg); err != nil {
		return SinkConfig{}, err
	}
	return cfg, nil
}

func Deliver(ctx context.Context, sink Sink, event domain.AuditEvent, client *http.Client) error {
	dest := strings.ToLower(strings.TrimSpace(sink.Destination))
	cfg, err := DecodeSinkConfig(sink)
	if err != nil {
		return err
	}
	payload, err := EncodeEvent(event)
	if err != nil {
		return err
	}
	switch dest {
	case "webhook":
		return deliverWebhook(ctx, sink, cfg, payload, client, event.EventID)
	case "syslog":
		return deliverSyslog(ctx, cfg, payload)
	default:
		return fmt.Errorf("unsupported audit export destination: %s", dest)
	}
}

func deliverWebhook(ctx context.Context, sink Sink, cfg SinkConfig, payload []byte, client *http.Client, eventID int64) error {
	url := strings.TrimSpace(cfg.WebhookURL)
	if url == "" {
		return errors.New("audit export webhook url required")
	}
	body := append(append([]byte{}, payload...), '\n')
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-ndjson")
	req.Header.Set("Idempotency-Key", fmt.Sprintf("%s:%d", sink.SinkID, eventID))
	req.Header.Set("X-Audit-Event-Id", fmt.Sprintf("%d", eventID))
	for key, value := range cfg.WebhookHeaders {
		name := strings.TrimSpace(key)
		if name == "" {
			continue
		}
		req.Header.Set(name, value)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook status %d", resp.StatusCode)
	}
	return nil
}

func deliverSyslog(ctx context.Context, cfg SinkConfig, payload []byte) error {
	addr := strings.TrimSpace(cfg.SyslogAddr)
	if addr == "" {
		return errors.New("audit export syslog addr required")
	}
	proto := strings.ToLower(strings.TrimSpace(cfg.SyslogProtocol))
	if proto == "" {
		proto = "udp"
	}
	if proto != "udp" && proto != "tcp" {
		return fmt.Errorf("unsupported syslog protocol: %s", proto)
	}

	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, proto, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	host, _ := os.Hostname()
	app := strings.TrimSpace(cfg.SyslogTag)
	if app == "" {
		app = "animus-audit"
	}
	msg := formatSyslogMessage(host, app, payload)
	_, err = conn.Write([]byte(msg))
	return err
}

func formatSyslogMessage(host, app string, payload []byte) string {
	return formatSyslogMessageAt(host, app, payload, time.Now().UTC())
}

func formatSyslogMessageAt(host, app string, payload []byte, now time.Time) string {
	const facilityUser = 1
	const severityNotice = 5
	pri := facilityUser*8 + severityNotice
	ts := now.UTC().Format(time.RFC3339Nano)
	return fmt.Sprintf("<%d>1 %s %s %s - - - %s\n", pri, ts, host, app, string(payload))
}
