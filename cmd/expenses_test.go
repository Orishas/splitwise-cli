package cmd

import (
	"strconv"
	"testing"

	"github.com/example/splitwise-cli/internal/api"
)

func members() []api.GroupMember {
	return []api.GroupMember{
		{ID: 1, FirstName: "Alice", LastName: "A"},
		{ID: 2, FirstName: "Bob", LastName: "B"},
		{ID: 3, FirstName: "Carol", LastName: "C"},
	}
}

func sumOwed(t *testing.T, shares []api.ShareParam) float64 {
	t.Helper()
	var sum float64
	for _, s := range shares {
		v, err := strconv.ParseFloat(s.OwedShare, 64)
		if err != nil {
			t.Fatalf("bad owed share %q: %v", s.OwedShare, err)
		}
		sum += v
	}
	return sum
}

func TestBuildEvenSharesPayerIsCustom(t *testing.T) {
	// Bob paid $90, split evenly 3 ways.
	shares, err := buildEvenShares(members(), "90.00", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shares) != 3 {
		t.Fatalf("expected 3 shares, got %d", len(shares))
	}
	var payerPaid string
	for _, s := range shares {
		if s.UserID == 2 {
			payerPaid = s.PaidShare
		} else if s.PaidShare != "0.00" {
			t.Errorf("non-payer %d has non-zero paid share: %s", s.UserID, s.PaidShare)
		}
		if s.OwedShare != "30.00" {
			t.Errorf("user %d: expected owed 30.00, got %s", s.UserID, s.OwedShare)
		}
	}
	if payerPaid != "90.00" {
		t.Errorf("payer paid share: want 90.00, got %s", payerPaid)
	}
}

func TestBuildEvenSharesRoundingSumsExactly(t *testing.T) {
	// 100 / 3 = 33.33, remainder 1 cent.
	shares, err := buildEvenShares(members(), "100.00", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	total := sumOwed(t, shares)
	if total != 100.00 {
		t.Errorf("owed shares sum to %.2f, want 100.00", total)
	}
}

func TestBuildEvenSharesUnknownPayer(t *testing.T) {
	if _, err := buildEvenShares(members(), "10.00", 999); err == nil {
		t.Fatal("expected error for non-member payer")
	}
}

func TestBuildEvenSharesEmptyGroup(t *testing.T) {
	if _, err := buildEvenShares(nil, "10.00", 1); err == nil {
		t.Fatal("expected error for empty group")
	}
}

func TestBuildExactSharesPayerAssignment(t *testing.T) {
	// Bob paid $100, Alice owes $60, Carol owes $40.
	shares, err := buildExactShares(members(), "100.00", 2, "Alice:60,Carol:40")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	owed := map[int64]string{}
	paid := map[int64]string{}
	for _, s := range shares {
		owed[s.UserID] = s.OwedShare
		paid[s.UserID] = s.PaidShare
	}
	if paid[2] != "100.00" {
		t.Errorf("payer paid: want 100.00, got %s", paid[2])
	}
	if owed[1] != "60.00" || owed[3] != "40.00" || owed[2] != "0.00" {
		t.Errorf("owed shares wrong: %v", owed)
	}
}

func TestBuildExactSharesMismatchedTotal(t *testing.T) {
	if _, err := buildExactShares(members(), "100.00", 1, "Alice:30,Bob:40"); err == nil {
		t.Fatal("expected error for total mismatch")
	}
}

func TestResolveMemberInGroup(t *testing.T) {
	g := &api.Group{Members: members()}

	cases := []struct {
		name string
		want int64
	}{
		{"alice", 1},
		{"BOB", 2},
		{"Carol C", 3},
	}
	for _, c := range cases {
		got, err := resolveMemberInGroup(g, c.name)
		if err != nil {
			t.Errorf("resolveMemberInGroup(%q): %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveMemberInGroup(%q) = %d, want %d", c.name, got, c.want)
		}
	}
	if _, err := resolveMemberInGroup(g, "Eve"); err == nil {
		t.Errorf("expected error for unknown name")
	}
}
