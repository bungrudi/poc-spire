package main

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestDenialDoesNotAcceptInfrastructureFailures(t *testing.T) {
	for _, err := range []error{nil, context.DeadlineExceeded,
		status.Error(codes.Unavailable, "no identity issued"),
		status.Error(codes.Internal, "attestor failed"),
		status.Error(codes.PermissionDenied, "permission to socket denied"),
	} {
		if denied(err) {
			t.Errorf("infrastructure failure accepted: %v", err)
		}
	}
	if !denied(status.Error(codes.PermissionDenied, "no identity issued")) {
		t.Fatal("policy rejection not recognized")
	}
}
