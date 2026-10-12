package main

import (
	"encoding/pem"
	"poc-spire/internal/testpki"
	"testing"
	"time"
)

func TestCompleteCABundleParserRejectsMalformedOrLeafMaterial(t *testing.T) {
	now := time.Now()
	ca, err := testpki.New(now)
	if err != nil {
		t.Fatal(err)
	}
	root := ca.Bundle().X509Authorities()[0]
	body := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: root.Raw})
	if !validBundle(body, now) {
		t.Fatal("authentic CA rejected")
	}
	for _, bad := range [][]byte{append(append([]byte{}, body...), []byte("broken")...), []byte("-----BEGIN PRIVATE KEY-----\ninvalid"), body[:len(body)-20]} {
		if validBundle(bad, now) {
			t.Fatal("partial/invalid bundle accepted")
		}
	}
	if validBundle(body, now.Add(2*time.Hour)) {
		t.Fatal("no current authority accepted")
	}
}
