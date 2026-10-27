package main

import (
	"testing"
	"time"
)

func TestStableOperationKeyDistinguishesSameLengthBodies(t *testing.T) {
	first := []byte(`{"queue":"q","payload":"a"}`)
	second := []byte(`{"queue":"q","payload":"b"}`)
	if len(first) != len(second) {
		t.Fatalf("test bodies must have equal length")
	}
	if stableOperationKey("POST", "/v1/queue/publish", first) == stableOperationKey("POST", "/v1/queue/publish", second) {
		t.Fatal("same-length request bodies must produce distinct operation keys")
	}
}

func TestDecideFollowUpUsesSignedDeliveryTime(t *testing.T) {
	received := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		matter Matter
		want   time.Time
	}{
		{"four hours", Matter{MatterID: "M-17", ReceivedAt: received, FollowUpAfter: 4 * time.Hour}, received.Add(4 * time.Hour)},
		{"next day", Matter{MatterID: "M-18", ReceivedAt: received, FollowUpAfter: 24 * time.Hour}, received.Add(24 * time.Hour)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := decideFollowUp(tt.matter)
			if got.DueAt != tt.want || got.Action != "request deadline status" {
				t.Fatalf("got %+v, want due %s and deadline action", got, tt.want)
			}
		})
	}
}
