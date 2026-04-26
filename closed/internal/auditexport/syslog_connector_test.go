package auditexport

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"
)

type bufferConn struct {
	buf bytes.Buffer
}

func (c *bufferConn) Read(p []byte) (int, error)         { return 0, nil }
func (c *bufferConn) Write(p []byte) (int, error)        { return c.buf.Write(p) }
func (c *bufferConn) Close() error                       { return nil }
func (c *bufferConn) LocalAddr() net.Addr                { return dummyAddr("local") }
func (c *bufferConn) RemoteAddr() net.Addr               { return dummyAddr("remote") }
func (c *bufferConn) SetDeadline(t time.Time) error      { return nil }
func (c *bufferConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *bufferConn) SetWriteDeadline(t time.Time) error { return nil }

type dummyAddr string

func (d dummyAddr) Network() string { return string(d) }
func (d dummyAddr) String() string  { return string(d) }

type stubDialer struct {
	conn net.Conn
	err  error
}

func (d stubDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	if d.err != nil {
		return nil, d.err
	}
	return d.conn, nil
}

func TestSyslogConnectorWritesMessage(t *testing.T) {
	conn := &bufferConn{}
	connector := SyslogConnector{
		Dialer: stubDialer{conn: conn},
		Now: func() time.Time {
			return time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
		},
		Hostname: func() string { return "unit-test" },
	}
	cfg := SinkConfig{SyslogAddr: "127.0.0.1:514", SyslogProtocol: "tcp", SyslogTag: "animus"}
	payload := []byte("{\"event\":\"ok\"}")

	result := connector.Deliver(context.Background(), cfg, payload)
	if result.Outcome != AttemptOutcomeSuccess {
		t.Fatalf("expected success, got %s", result.Outcome)
	}
	msg := conn.buf.String()
	if msg == "" {
		t.Fatalf("expected message written")
	}
	if !bytes.Contains(conn.buf.Bytes(), payload) {
		t.Fatalf("expected payload in syslog message")
	}
	if msg[len(msg)-1] != '\n' {
		t.Fatalf("expected newline-terminated message")
	}
}

func TestSyslogConnectorInvalidProtocol(t *testing.T) {
	connector := SyslogConnector{}
	cfg := SinkConfig{SyslogAddr: "127.0.0.1:514", SyslogProtocol: "icmp"}
	result := connector.Deliver(context.Background(), cfg, []byte("{}"))
	if result.Outcome != AttemptOutcomePermanentFailure {
		t.Fatalf("expected permanent failure, got %s", result.Outcome)
	}
}

func TestSyslogConnectorDialFailureIsRetry(t *testing.T) {
	connector := SyslogConnector{Dialer: stubDialer{err: context.DeadlineExceeded}}
	cfg := SinkConfig{SyslogAddr: "127.0.0.1:514", SyslogProtocol: "udp"}
	result := connector.Deliver(context.Background(), cfg, []byte("{}"))
	if result.Outcome != AttemptOutcomeRetry {
		t.Fatalf("expected retry, got %s", result.Outcome)
	}
}
