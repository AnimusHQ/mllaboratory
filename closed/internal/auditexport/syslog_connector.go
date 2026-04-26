package auditexport

import (
	"context"
	"net"
	"os"
	"strings"
	"time"
)

type SyslogDialer interface {
	DialContext(ctx context.Context, network, addr string) (net.Conn, error)
}

type SyslogConnector struct {
	Dialer   SyslogDialer
	Now      func() time.Time
	Hostname func() string
}

func (c SyslogConnector) Deliver(ctx context.Context, cfg SinkConfig, payload []byte) DeliveryResult {
	addr := strings.TrimSpace(cfg.SyslogAddr)
	if addr == "" {
		return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "syslog_addr_required"}
	}
	proto := strings.ToLower(strings.TrimSpace(cfg.SyslogProtocol))
	if proto == "" {
		proto = "udp"
	}
	if proto != "udp" && proto != "tcp" {
		return DeliveryResult{Outcome: AttemptOutcomePermanentFailure, Error: "syslog_protocol_invalid"}
	}
	dialer := c.Dialer
	if dialer == nil {
		dialer = &net.Dialer{}
	}
	conn, err := dialer.DialContext(ctx, proto, addr)
	if err != nil {
		return DeliveryResult{Outcome: AttemptOutcomeRetry, Error: "syslog_dial_failed"}
	}
	defer conn.Close()

	host := ""
	if c.Hostname != nil {
		host = strings.TrimSpace(c.Hostname())
	}
	if host == "" {
		host, _ = os.Hostname()
	}
	if host == "" {
		host = "localhost"
	}
	app := strings.TrimSpace(cfg.SyslogTag)
	if app == "" {
		app = "animus-audit"
	}
	now := time.Now().UTC()
	if c.Now != nil {
		now = c.Now().UTC()
	}
	msg := formatSyslogMessageAt(host, app, payload, now)
	if _, err := conn.Write([]byte(msg)); err != nil {
		return DeliveryResult{Outcome: AttemptOutcomeRetry, Error: "syslog_write_failed"}
	}
	return DeliveryResult{Outcome: AttemptOutcomeSuccess}
}
