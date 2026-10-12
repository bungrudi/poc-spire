//go:build !kafkatest

package main

import (
	"context"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

func fixtureRun(context.Context, *workloadapi.X509Source) bool { return false }
