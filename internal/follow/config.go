package follow

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/soroban-fleetwatch/fleetwatch/internal/jsoninput"
	"github.com/soroban-fleetwatch/fleetwatch/internal/source"
)

type Config struct {
	SchemaVersion           int    `json:"schema_version"`
	RPCURL                  string `json:"rpc_url"`
	Baseline                string `json:"baseline"`
	State                   string `json:"state"`
	Listen                  string `json:"listen"`
	PollIntervalSeconds     int    `json:"poll_interval_seconds"`
	BatchSize               int    `json:"batch_size"`
	HeadDelayLedgers        uint32 `json:"head_delay_ledgers"`
	MaxEndpointLag          uint32 `json:"max_endpoint_lag"`
	MaxRetainedLedgers      int    `json:"max_retained_ledgers"`
	MaxRetainedPayloadBytes int64  `json:"max_retained_payload_bytes"`
}

func (c Config) Limits() Limits { return Limits{c.MaxRetainedLedgers, c.MaxRetainedPayloadBytes} }
func (c Config) Validate() error {
	bad := errors.New("invalid follow configuration")
	if c.SchemaVersion != 1 || c.PollIntervalSeconds < 1 || c.PollIntervalSeconds > 60 || c.BatchSize < 1 || c.BatchSize > 16 || c.HeadDelayLedgers > 8 || c.MaxEndpointLag < c.HeadDelayLedgers+1 || c.MaxEndpointLag > 100000 || c.Limits().Validate() != nil {
		return bad
	}
	if _, err := source.NewHTTPRPC(c.RPCURL); err != nil {
		return bad
	}
	host, port, err := net.SplitHostPort(c.Listen)
	n, e := strconv.Atoi(port)
	if err != nil || host != "127.0.0.1" || e != nil || n < 0 || n > 65535 || strconv.Itoa(n) != port {
		return bad
	}
	for _, p := range []string{c.Baseline, c.State} {
		if strings.TrimSpace(p) == "" || len(p) > 4096 || !utf8.ValidString(p) {
			return bad
		}
		for _, r := range p {
			if unicode.IsControl(r) {
				return bad
			}
		}
	}
	return nil
}
func LoadConfig(path string) (Config, error) {
	var c Config
	bad := errors.New("invalid follow configuration")
	f, err := os.Open(path)
	if err != nil {
		return c, bad
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(raw) > 16384 || jsoninput.Decode(bytes.NewReader(raw), &c) != nil || c.Validate() != nil {
		return Config{}, bad
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return Config{}, bad
	}
	for _, key := range []string{"schema_version", "rpc_url", "baseline", "state", "listen", "poll_interval_seconds", "batch_size", "head_delay_ledgers", "max_endpoint_lag", "max_retained_ledgers", "max_retained_payload_bytes"} {
		value, ok := fields[key]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return Config{}, bad
		}
	}
	configPath, err := filepath.Abs(path)
	if err != nil {
		return Config{}, bad
	}
	if !filepath.IsAbs(c.Baseline) {
		c.Baseline = filepath.Join(filepath.Dir(configPath), c.Baseline)
	}
	if !filepath.IsAbs(c.State) {
		c.State = filepath.Join(filepath.Dir(configPath), c.State)
	}
	c.Baseline = filepath.Clean(c.Baseline)
	c.State = filepath.Clean(c.State)
	if c.State == c.Baseline || c.State == configPath {
		return Config{}, bad
	}
	return c, nil
}
