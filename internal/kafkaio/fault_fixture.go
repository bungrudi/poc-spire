//go:build kafkatest

package kafkaio

import (
	"context"
	"errors"
	"flag"
	"github.com/twmb/franz-go/pkg/kgo"
	"os"
)

var ackBlock = flag.String("fixture-ack-block", "", "private host-controlled acknowledgement gate; test build only")
var processingBlock = flag.String("fixture-processing-block", "", "private host-controlled pre-decision gate; test build only")

func configureFixtureFaults(c *Consumer) {
	if *ackBlock != "" {
		c.beforeCommit = func(context.Context, *Delivery) error {
			if _, err := os.Stat(*ackBlock); err == nil {
				return errors.New("fixture_ack_uncertain")
			}
			return nil
		}
	}
	if *processingBlock != "" {
		c.engine.BeforeDecision = func() bool { _, err := os.Stat(*processingBlock); return err != nil }
	}
}

func (c *Consumer) FixtureGroupPoll(ctx context.Context) error {
	fetches := c.client.PollRecords(ctx, 1)
	return fetches.Err()
}

var _ = kgo.DisableAutoCommit
