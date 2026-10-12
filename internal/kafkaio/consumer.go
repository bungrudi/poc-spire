package kafkaio

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"poc-spire/internal/kafkacmd"
	"poc-spire/internal/ledger"
)

type Consumer struct {
	client *kgo.Client
	book   *ledger.Ledger
	cfg    Config
	engine Engine
	held   *Delivery
}

func NewConsumer(cfg Config, book *ledger.Ledger, source x509bundle.Source, audit io.Writer) (*Consumer, error) {
	opts, err := options(cfg)
	if err != nil {
		return nil, err
	}
	consumer := &Consumer{book: book, cfg: cfg, engine: Engine{Book: book, Policy: cfg.Policy(), Source: source, Audit: audit}}
	pause := func(client *kgo.Client) { client.PauseFetchPartitions(map[string][]int32{cfg.Policy().Topic: {0}}) }
	revoke := func(_ context.Context, client *kgo.Client, _ map[string][]int32) {
		pause(client)
		book.SetKafkaAssignment(false)
	}
	opts = append(opts, kgo.ConsumerGroup(cfg.Group()), kgo.ConsumeTopics(cfg.Policy().Topic), kgo.DisableAutoCommit(), kgo.BlockRebalanceOnPoll(), kgo.Balancers(kgo.RangeBalancer()), kgo.FetchMaxBytes(65536), kgo.FetchMaxPartitionBytes(65536), kgo.MaxConcurrentFetches(1), kgo.FetchMaxWait(250*time.Millisecond), kgo.OnPartitionsAssigned(func(_ context.Context, client *kgo.Client, partitions map[string][]int32) {
		pause(client)
		assigned := len(partitions) == 1 && len(partitions[cfg.Policy().Topic]) == 1 && partitions[cfg.Policy().Topic][0] == 0
		book.SetKafkaAssignment(assigned)
	}), kgo.OnPartitionsRevoked(revoke), kgo.OnPartitionsLost(revoke))
	consumer.client, err = kgo.NewClient(opts...)
	if err != nil {
		return nil, err
	}
	return consumer, nil
}
func (c *Consumer) Run(ctx context.Context) { c.engine.Run(ctx, c) }
func (c *Consumer) Close() {
	c.book.SetKafkaAssignment(false)
	c.client.AllowRebalance()
	c.client.Close()
}
func (c *Consumer) Pause() {
	c.client.PauseFetchPartitions(map[string][]int32{c.cfg.Policy().Topic: {0}})
}
func (c *Consumer) Resume() {
	c.client.AllowRebalance()
	meta, enabled := c.book.KafkaMetadata()
	if enabled && meta.ConsumerReady {
		c.client.ResumeFetchPartitions(map[string][]int32{c.cfg.Policy().Topic: {0}})
	}
}
func (c *Consumer) delivery(record *kgo.Record) *Delivery {
	headers := make([]kafkacmd.Header, len(record.Headers))
	for i, h := range record.Headers {
		headers[i] = kafkacmd.Header{Name: h.Key, Value: h.Value}
	}
	member, generation := c.client.GroupMetadata()
	return &Delivery{Record: kafkacmd.Record{Topic: record.Topic, Key: record.Key, Value: record.Value, Headers: headers}, Partition: record.Partition, Offset: record.Offset, LeaderEpoch: record.LeaderEpoch, AssignmentEpoch: c.book.KafkaEpoch(), MemberID: member, Generation: generation}
}
func (c *Consumer) Poll(ctx context.Context) (*Delivery, error) {
	if c.held != nil {
		record := c.held
		c.held = nil
		return record, nil
	}
	fetches := c.client.PollRecords(ctx, 1)
	if err := fetches.Err(); err != nil {
		return nil, err
	}
	records := fetches.Records()
	if len(records) == 0 {
		return nil, nil
	}
	return c.delivery(records[0]), nil
}
func (c *Consumer) Heartbeat(ctx context.Context) error {
	c.Pause()
	c.client.AllowRebalance()
	defer c.client.AllowRebalance()
	// The pause is retained across assignment callbacks. Preserve an unexpected
	// in-flight fetched record rather than silently skipping it.
	if c.held != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			return nil
		}
	}
	fetches := c.client.PollRecords(ctx, 1)
	if err := fetches.Err(); err != nil {
		return err
	}
	records := fetches.Records()
	if len(records) != 0 {
		c.held = c.delivery(records[0])
	}
	return nil
}
func (c *Consumer) Assigned(d *Delivery) bool {
	meta, enabled := c.book.KafkaMetadata()
	member, generation := c.client.GroupMetadata()
	return enabled && meta.ConsumerReady && c.book.KafkaEpoch() == d.AssignmentEpoch && d.Partition == 0 && d.Record.Topic == c.cfg.Policy().Topic && member == d.MemberID && generation == d.Generation && member != "" && generation >= 0
}
func (c *Consumer) Commit(ctx context.Context, d *Delivery) error {
	if !c.Assigned(d) {
		return errors.New("assignment_lost")
	}
	request := kmsg.NewPtrOffsetCommitRequest()
	request.Group = c.cfg.Group()
	request.MemberID = d.MemberID
	request.Generation = d.Generation
	partition := kmsg.NewOffsetCommitRequestTopicPartition()
	partition.Partition = d.Partition
	partition.Offset = d.Offset + 1
	partition.LeaderEpoch = d.LeaderEpoch
	request.Topics = []kmsg.OffsetCommitRequestTopic{{Topic: d.Record.Topic, Partitions: []kmsg.OffsetCommitRequestTopicPartition{partition}}}
	// Send the captured generation/member, never a later group's generation.
	// A reassignment therefore cannot authorize this older record's commit.
	response, err := request.RequestWith(ctx, c.client)
	if err != nil {
		return err
	}
	if response == nil || len(response.Topics) != 1 || response.Topics[0].Topic != d.Record.Topic || len(response.Topics[0].Partitions) != 1 || response.Topics[0].Partitions[0].Partition != d.Partition {
		return errors.New("uncorrelated_commit_response")
	}
	return kerr.ErrorForCode(response.Topics[0].Partitions[0].ErrorCode)
}
