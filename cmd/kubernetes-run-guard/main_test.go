package main

import "testing"

func TestDestructiveTerminationBindsOnlySanitizedRunRoleAndPod(t *testing.T) {
	want := "poc-spire:destructive-startup-state:spire-server:abcdef012345:original-uid"
	if terminationMessage("spire-server", "abcdef012345", "original-uid") != want {
		t.Fatal("source state signal lost ownership")
	}
	if destructiveExit != 78 {
		t.Fatal("observer startup exit contract changed")
	}
}
