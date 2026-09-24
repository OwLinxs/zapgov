package main

import (
	"errors"
	"testing"
	"zapgov/backend/internal/messaging"
)

func TestDeliveryClassification(t *testing.T) {
	cases := []struct {
		err   error
		state string
	}{{nil, "sent"}, {messaging.ErrOptedOut, "skipped"}, {messaging.ErrProviderUnavailable, "failed"}, {&messaging.ProviderError{Status: 429}, "retry"}, {&messaging.ProviderError{Status: 400}, "failed"}, {&messaging.ProviderError{Status: 503}, "uncertain"}, {errors.New("timeout after sending request"), "uncertain"}}
	for _, tc := range cases {
		if got := deliveryState(tc.err); got != tc.state {
			t.Errorf("%v: %s want %s", tc.err, got, tc.state)
		}
	}
}
