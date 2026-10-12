// Package moneyreader reads outcomes using only the UI's own observer source
// and the existing exact-Money SPIFFE mTLS verification policy.
package moneyreader

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"poc-spire/internal/identity"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type Observer struct {
	Target string
	SVID   x509svid.Source
	Bundle x509bundle.Source
}

func decode(raw []byte, out any) error {
	if _, err := kafkacmd.StrictJSON(raw); err != nil {
		return errors.New("invalid_money_evidence")
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if d.Decode(out) != nil {
		return errors.New("invalid_money_evidence")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid_money_evidence")
	}
	return nil
}
func (o Observer) get(ctx context.Context, path string) ([]byte, int, error) {
	if o.SVID == nil || o.Bundle == nil {
		return nil, 0, errors.New("observer_identity_unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	client, transport := identity.HTTPClient(identity.ClientTLS(o.SVID, o.Bundle, identity.Money))
	defer transport.CloseIdleConnections()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Target+path, nil)
	if err != nil {
		return nil, 0, errors.New("money_read_unavailable")
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, 0, errors.New("money_read_unavailable")
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8193))
	if err != nil || len(raw) > 8192 {
		return nil, response.StatusCode, errors.New("invalid_money_evidence")
	}
	return raw, response.StatusCode, nil
}
func (o Observer) Metadata(ctx context.Context) (ledger.KafkaMetadata, error) {
	raw, code, err := o.get(ctx, "/kafka/outcomes")
	if err != nil || code != 200 {
		return ledger.KafkaMetadata{}, errors.New("money_metadata_unavailable")
	}
	var m ledger.KafkaMetadata
	if decode(raw, &m) != nil || !kafkacmd.Hex32.MatchString(m.InstanceID) || !kafkacmd.Hex32.MatchString(m.StreamID) {
		return ledger.KafkaMetadata{}, errors.New("invalid_money_metadata")
	}
	return m, nil
}
func (o Observer) Receipt(ctx context.Context, digest string) (ledger.Receipt, ledger.KafkaMetadata, int, error) {
	if !kafkacmd.Hex64.MatchString(digest) {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, 0, errors.New("invalid_record_digest")
	}
	raw, code, err := o.get(ctx, "/kafka/outcomes/"+digest)
	if err != nil {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, code, err
	}
	if code == 404 {
		var m ledger.KafkaMetadata
		if decode(raw, &m) != nil || !kafkacmd.Hex32.MatchString(m.InstanceID) || !kafkacmd.Hex32.MatchString(m.StreamID) {
			return ledger.Receipt{}, m, code, errors.New("invalid_money_metadata")
		}
		return ledger.Receipt{}, m, code, nil
	}
	if code != 200 {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, code, errors.New("money_receipt_unavailable")
	}
	var receipt ledger.Receipt
	if decode(raw, &receipt) != nil || receipt.Digest != digest || !kafkacmd.Hex32.MatchString(receipt.InstanceID) || receipt.ObservedAt < 0 || receipt.RetainUntil-receipt.ObservedAt != 180 {
		return ledger.Receipt{}, ledger.KafkaMetadata{}, code, errors.New("uncorrelated_money_receipt")
	}
	return receipt, ledger.KafkaMetadata{InstanceID: receipt.InstanceID}, code, nil
}
