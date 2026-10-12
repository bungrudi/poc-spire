package kafkacaller

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"poc-spire/internal/kafkacmd"
	"strings"
)

type Sender struct {
	mu         sync.Mutex
	Store      *Store
	Source     x509svid.Source
	Publish    func(context.Context, kafkacmd.Record) error
	NoIdentity func(context.Context) error
	Clock      func() time.Time
}

func (s *Sender) now() time.Time {
	if s.Clock != nil {
		return s.Clock()
	}
	return time.Now()
}
func NoIdentity(socket string) func(context.Context) error {
	return func(ctx context.Context) error {
		client, err := workloadapi.New(ctx, workloadapi.WithAddr("unix://"+socket))
		if err != nil {
			return errors.New("workload_api_unavailable")
		}
		defer client.Close()
		fetch, cancel := context.WithTimeout(ctx, 4*time.Second)
		defer cancel()
		_, err = client.FetchX509SVIDs(fetch)
		if err == nil {
			return errors.New("unexpected_identity_issued")
		}
		if status.Code(err) != codes.PermissionDenied || !strings.Contains(status.Convert(err).Message(), "no identity issued") {
			return errors.New("workload_api_unavailable")
		}
		return nil
	}
}
func (s *Sender) Send(ctx context.Context, id, scenario, instance string) (Metadata, int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		return Metadata{}, 503, ctx.Err()
	}
	descriptor, code, err := s.Store.allocate(id, scenario, instance, s.now())
	if err != nil {
		return metadata(descriptor, s.Store.policy.StreamID), code, err
	}
	if scenario == "kafka-unidentified" {
		if s.NoIdentity == nil {
			return metadata(descriptor, s.Store.policy.StreamID), 503, errors.New("identity_probe_unavailable")
		}
		if err := s.NoIdentity(ctx); err != nil {
			return metadata(descriptor, s.Store.policy.StreamID), 503, err
		}
	}
	if s.now().Unix() >= descriptor.Context.Deadline {
		return metadata(descriptor, s.Store.policy.StreamID), 409, errors.New("descriptor_expired")
	}
	var variant Variant
	if len(descriptor.Variants) > 0 {
		old := descriptor.Variants[len(descriptor.Variants)-1]
		if scenario == "kafka-unidentified" || old.ExpiresAt-s.now().Unix() > 5 {
			variant = old
		}
	}
	if variant.Digest == "" {
		if len(descriptor.Variants) >= 4 {
			return metadata(descriptor, s.Store.policy.StreamID), 503, errors.New("variant_capacity_exhausted")
		}
		signing, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		for {
			if s.now().Unix() >= descriptor.Context.Deadline {
				return metadata(descriptor, s.Store.policy.StreamID), 409, errors.New("descriptor_expired")
			}
			var svid *x509svid.SVID
			if scenario != "kafka-unidentified" && s.Source != nil {
				svid, _ = s.Source.GetX509SVID()
			}
			record, fail := kafkacmd.FixedRecord(svid, descriptor.Context, id, scenario, s.now())
			if fail == nil {
				expires := min(s.now().Unix()+30, descriptor.Context.Deadline)
				if scenario != "kafka-unidentified" {
					parsed, f := kafkacmd.Parse(record)
					if f != nil {
						return metadata(descriptor, s.Store.policy.StreamID), 503, f
					}
					expires = parsed.Message.ExpiresAt
				}
				variant = Variant{record, kafkacmd.Digest(record), expires, "publish_unknown"}
				break
			}
			if !fail.Transient {
				return metadata(descriptor, s.Store.policy.StreamID), 400, fail
			}
			select {
			case <-signing.Done():
				return metadata(descriptor, s.Store.policy.StreamID), 503, errors.New("signing_unavailable")
			case <-time.After(100 * time.Millisecond):
			}
		}
		// Exact bytes and an ambiguous-publication marker are durable before calling
		// the broker. A crash in any later instruction remains potentially submitted.
		_, err = s.Store.mutate(id, func(d *Descriptor) error {
			if d.Context != descriptor.Context || d.MoneyInstanceID != instance || len(d.Variants) >= 4 {
				return errors.New("descriptor_changed")
			}
			d.Variants = append(d.Variants, variant)
			return nil
		})
		if err != nil {
			return metadata(descriptor, s.Store.policy.StreamID), 503, err
		}
	}
	// An acknowledged old variant also becomes ambiguous before another send;
	// failure cannot downgrade a possible publication to not_submitted.
	_, err = s.Store.mutate(id, func(d *Descriptor) error {
		for i := range d.Variants {
			if d.Variants[i].Digest == variant.Digest {
				d.Variants[i].Publication = "publish_unknown"
				return nil
			}
		}
		return errors.New("expected_variant_missing")
	})
	if err != nil {
		return Metadata{}, 503, err
	}
	if ctx.Err() != nil || s.now().Unix() >= descriptor.Context.Deadline {
		return Metadata{}, 409, errors.New("descriptor_expired_or_cancelled")
	}
	if s.Publish == nil {
		return Metadata{}, 503, errors.New("broker_unavailable")
	}
	err = s.Publish(ctx, variant.Record)
	if err == nil {
		_, persistErr := s.Store.mutate(id, func(d *Descriptor) error {
			for i := range d.Variants {
				if d.Variants[i].Digest == variant.Digest {
					d.Variants[i].Publication = "queued"
					return nil
				}
			}
			return errors.New("expected_variant_missing")
		})
		if persistErr != nil {
			return Metadata{}, 503, persistErr
		}
	}
	current, lookupCode, lookupErr := s.Store.Lookup(id)
	if lookupErr != nil || lookupCode != 200 {
		return Metadata{}, 503, errors.New("publication_metadata_unavailable")
	}
	// The caller never reports Money success. Both queued and publish_unknown are
	// successful control diagnostics requiring authenticated outcome polling.
	return current, 200, nil
}
