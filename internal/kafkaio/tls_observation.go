package kafkaio

import (
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// A connection hook observes the default franz-go dial/verification/reuse path.
// stderr preserves fixture stdout's fixed JSON/private prepared-record protocol.
type tlsObservation struct {
	audit io.Writer
	mutex sync.Mutex
}

func (o *tlsObservation) OnBrokerConnect(_ kgo.BrokerMetadata, _ time.Duration, connection net.Conn, err error) {
	if err != nil || connection == nil {
		return
	}
	secure, ok := connection.(*tls.Conn)
	if !ok {
		return
	}
	state := secure.ConnectionState()
	if !state.HandshakeComplete || len(state.VerifiedChains) == 0 {
		return
	}
	observation := map[string]any{"event": "broker_tls_connected", "at_utc": time.Now().UTC().Format(time.RFC3339Nano),
		"local": connection.LocalAddr().String(), "remote": connection.RemoteAddr().String(),
		"tls_version": tls.VersionName(state.Version), "tls_session_resumed": state.DidResume, "verified_peer_name": state.ServerName}
	o.mutex.Lock()
	defer o.mutex.Unlock()
	json.NewEncoder(o.audit).Encode(observation)
}

var _ kgo.HookBrokerConnect = (*tlsObservation)(nil)
