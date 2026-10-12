//go:build kafkatest

package main

import (
	"context"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"poc-spire/internal/kafkafixture"
)

func fixtureRun(ctx context.Context, source *workloadapi.X509Source) bool {
	return kafkafixture.Run(ctx, source)
}
