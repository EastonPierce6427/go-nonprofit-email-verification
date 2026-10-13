package main

import (
	"context"
	"testing"
)

type fakeGateway struct {
	sent    int
	created int
}

func (g *fakeGateway) createUser(context.Context, string, string, string) error {
	g.created++
	return nil
}
func (g *fakeGateway) send(context.Context, string, string, string, string) (string, error) {
	g.sent++
	return "mail-1", nil
}
func TestVerifiedDeliveryAndCampaignReport(t *testing.T) {
	for _, tc := range []struct {
		name, kind                                       string
		cents                                            int
		verify                                           bool
		wantSent, wantReceipts, wantCents, wantReminders int
	}{
		{"unverified receipt", "receipt", 2500, false, 1, 0, 0, 0},
		{"verified receipt", "receipt", 2500, true, 2, 1, 2500, 0},
		{"verified reminder", "reminder", 0, true, 2, 0, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := &fakeGateway{}
			f := newFlow(g, "http://localhost:8080")
			if _, err := f.signup(context.Background(), "donor@example.org", "Donor"); err != nil {
				t.Fatal(err)
			}
			if tc.verify {
				for token := range f.pending {
					if err := f.verify(token); err != nil {
						t.Fatal(err)
					}
				}
			}
			_, err := f.deliver(context.Background(), tc.kind, "donor@example.org", "spring", tc.cents)
			if (err == nil) != tc.verify {
				t.Fatalf("delivery error = %v", err)
			}
			got := f.report("spring")
			if g.sent != tc.wantSent || got.Receipts != tc.wantReceipts || got.DonatedCents != tc.wantCents || got.Reminders != tc.wantReminders {
				t.Fatalf("sent=%d report=%+v", g.sent, got)
			}
			if other := f.report("autumn"); other != (campaignReport{}) {
				t.Fatalf("other campaign=%+v", other)
			}
		})
	}
}
