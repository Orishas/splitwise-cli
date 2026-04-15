package api

import (
	"encoding/json"
	"net/url"
	"testing"
)

func TestFormatAPIErrors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "nil",
			in:   `null`,
			want: "",
		},
		{
			name: "empty",
			in:   `{}`,
			want: "",
		},
		{
			name: "base array",
			in:   `{"base": ["account disabled"]}`,
			want: "account disabled",
		},
		{
			name: "field with messages",
			in:   `{"cost": ["must be positive", "must be a number"]}`,
			want: "cost: must be positive; cost: must be a number",
		},
		{
			name: "multiple fields sorted",
			in:   `{"cost": ["too low"], "base": ["generic"]}`,
			want: "generic; cost: too low",
		},
		{
			name: "scalar string",
			in:   `{"reason": "forbidden"}`,
			want: "reason: forbidden",
		},
		{
			name: "nested map",
			in:   `{"users": {"0": ["invalid"]}}`,
			want: "users: 0: invalid",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal([]byte(c.in), &m); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			got := formatAPIErrors(m)
			if got != c.want {
				t.Errorf("formatAPIErrors(%s) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestEncodeSharesFlattensWithDoubleUnderscores(t *testing.T) {
	params := url.Values{}
	encodeShares(params, []ShareParam{
		{UserID: 10, PaidShare: "30.00", OwedShare: "10.00"},
		{UserID: 20, PaidShare: "0.00", OwedShare: "20.00"},
	})
	expected := map[string]string{
		"users__0__user_id":    "10",
		"users__0__paid_share": "30.00",
		"users__0__owed_share": "10.00",
		"users__1__user_id":    "20",
		"users__1__paid_share": "0.00",
		"users__1__owed_share": "20.00",
	}
	for k, want := range expected {
		if got := params.Get(k); got != want {
			t.Errorf("params[%s] = %q, want %q", k, got, want)
		}
	}
}

func TestEncodeCreateExpenseEvenSplit(t *testing.T) {
	p := CreateExpenseParams{
		Description:  "Dinner",
		Cost:         "40.00",
		CurrencyCode: "EUR",
		GroupID:      99,
		SplitEqually: true,
		Date:         "2026-04-01",
		Details:      "note",
	}
	v := encodeCreateExpense(p, false)
	if v.Get("split_equally") != "true" {
		t.Errorf("split_equally: want true, got %q", v.Get("split_equally"))
	}
	if v.Get("group_id") != "99" {
		t.Errorf("group_id: want 99, got %q", v.Get("group_id"))
	}
	if v.Get("payment") != "" {
		t.Errorf("payment should not be set for regular expenses: %q", v.Get("payment"))
	}
	if v.Get("date") != "2026-04-01" || v.Get("details") != "note" {
		t.Errorf("date/details not propagated: %v", v)
	}
}

func TestEncodeCreateExpensePaymentSetsFlag(t *testing.T) {
	p := CreateExpenseParams{
		Description: "Payment",
		Cost:        "50.00",
		GroupID:     0,
		Shares: []ShareParam{
			{UserID: 1, PaidShare: "50.00", OwedShare: "0.00"},
			{UserID: 2, PaidShare: "0.00", OwedShare: "50.00"},
		},
	}
	v := encodeCreateExpense(p, true)
	if v.Get("payment") != "true" {
		t.Errorf("payment: want true, got %q", v.Get("payment"))
	}
	if v.Get("group_id") != "0" {
		t.Errorf("non-group payment: want group_id=0, got %q", v.Get("group_id"))
	}
	if v.Get("users__0__user_id") != "1" || v.Get("users__1__user_id") != "2" {
		t.Errorf("user IDs not encoded: %v", v)
	}
}

func TestEncodeCreateExpenseOmitsEmptyOptionals(t *testing.T) {
	p := CreateExpenseParams{
		Description:  "Rent",
		Cost:         "1000.00",
		GroupID:      5,
		SplitEqually: true,
	}
	v := encodeCreateExpense(p, false)
	for _, k := range []string{"date", "details", "category_id", "currency_code"} {
		if v.Has(k) {
			t.Errorf("empty optional %q should be omitted, got %q", k, v.Get(k))
		}
	}
}
