package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/example/splitwise-cli/internal/auth"
)

const baseURL = "https://secure.splitwise.com/api/v3.0"

// Client is a Splitwise API client.
type Client struct {
	http  *http.Client
	token string
}

// New creates a new authenticated API client.
func New() (*Client, error) {
	token, err := auth.LoadToken()
	if err != nil {
		return nil, err
	}
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
		token: token,
	}, nil
}

func (c *Client) get(path string, params url.Values) ([]byte, error) {
	u := baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	return c.do(req)
}

func (c *Client) post(path string, params url.Values) ([]byte, error) {
	u := baseURL + path
	var body io.Reader
	if len(params) > 0 {
		body = strings.NewReader(params.Encode())
	}
	req, err := http.NewRequest("POST", u, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("unauthorized — run `splitwise auth` to re-authenticate")
	}
	if resp.StatusCode == 403 {
		return nil, fmt.Errorf("forbidden — you don't have access to this resource")
	}
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("not found")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// ---------- Types ----------

// User represents a Splitwise user.
type User struct {
	ID                 int64              `json:"id"`
	FirstName          string             `json:"first_name"`
	LastName           string             `json:"last_name"`
	Email              string             `json:"email"`
	RegistrationStatus string             `json:"registration_status"`
	DefaultCurrency    string             `json:"default_currency,omitempty"`
	Locale             string             `json:"locale,omitempty"`
	NotificationsCount int                `json:"notifications_count,omitempty"`
	Picture            *UserPicture       `json:"picture,omitempty"`
}

type UserPicture struct {
	Small  string `json:"small"`
	Medium string `json:"medium"`
	Large  string `json:"large"`
}

// Balance represents a currency balance.
type Balance struct {
	CurrencyCode string `json:"currency_code"`
	Amount       string `json:"amount"`
}

// Debt represents a debt between two users.
type Debt struct {
	From         int64  `json:"from"`
	To           int64  `json:"to"`
	Amount       string `json:"amount"`
	CurrencyCode string `json:"currency_code"`
}

// Group represents a Splitwise group.
type Group struct {
	ID                 int64         `json:"id"`
	Name               string        `json:"name"`
	GroupType          string        `json:"group_type"`
	UpdatedAt          string        `json:"updated_at"`
	SimplifyByDefault  bool          `json:"simplify_by_default"`
	Members            []GroupMember `json:"members"`
	OriginalDebts      []Debt        `json:"original_debts"`
	SimplifiedDebts    []Debt        `json:"simplified_debts"`
	InviteLink         string        `json:"invite_link,omitempty"`
}

// GroupMember is a user with group-specific balance info.
type GroupMember struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Balance   []Balance `json:"balance"`
}

// Friend represents a Splitwise friend.
type Friend struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
	Balance   []Balance `json:"balance"`
	Groups    []FriendGroup `json:"groups"`
}

type FriendGroup struct {
	GroupID int64     `json:"group_id"`
	Balance []Balance `json:"balance"`
}

// Expense represents a Splitwise expense.
type Expense struct {
	ID            int64          `json:"id"`
	GroupID       *int64         `json:"group_id"`
	Description   string         `json:"description"`
	Cost          string         `json:"cost"`
	CurrencyCode  string         `json:"currency_code"`
	Date          string         `json:"date"`
	Payment       bool           `json:"payment"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
	DeletedAt     *string        `json:"deleted_at"`
	Details       *string        `json:"details"`
	Repayments    []Repayment    `json:"repayments"`
	Users         []ExpenseShare `json:"users"`
	Category      *Category      `json:"category"`
	CreatedBy     *User          `json:"created_by"`
}

type Repayment struct {
	From   int64  `json:"from"`
	To     int64  `json:"to"`
	Amount string `json:"amount"`
}

type ExpenseShare struct {
	UserID     int64  `json:"user_id"`
	PaidShare  string `json:"paid_share"`
	OwedShare  string `json:"owed_share"`
	NetBalance string `json:"net_balance"`
	User       *User  `json:"user"`
}

type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ---------- API Methods ----------

// GetCurrentUser returns the authenticated user.
func (c *Client) GetCurrentUser() (*User, error) {
	data, err := c.get("/get_current_user", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		User *User `json:"user"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.User, nil
}

// GetGroups returns all groups for the current user.
func (c *Client) GetGroups() ([]Group, error) {
	data, err := c.get("/get_groups", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Groups []Group `json:"groups"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Groups, nil
}

// GetGroup returns a single group by ID.
func (c *Client) GetGroup(id int64) (*Group, error) {
	data, err := c.get(fmt.Sprintf("/get_group/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Group *Group `json:"group"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Group, nil
}

// GetFriends returns all friends for the current user.
func (c *Client) GetFriends() ([]Friend, error) {
	data, err := c.get("/get_friends", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Friends []Friend `json:"friends"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Friends, nil
}

// GetExpensesParams holds query parameters for listing expenses.
// All fields are optional; zero-values are omitted from the request.
type GetExpensesParams struct {
	GroupID       int64
	FriendID      int64
	DatedAfter    string // ISO 8601 date or datetime
	DatedBefore   string
	UpdatedAfter  string
	UpdatedBefore string
	Limit         int
	Offset        int
	// Visible, when non-nil, is sent to the server. The Splitwise API
	// filters out deleted expenses when this is true.
	Visible *bool
}

// GetExpenses returns expenses matching the given criteria.
func (c *Client) GetExpenses(p GetExpensesParams) ([]Expense, error) {
	params := url.Values{}
	if p.GroupID > 0 {
		params.Set("group_id", fmt.Sprintf("%d", p.GroupID))
	}
	if p.FriendID > 0 {
		params.Set("friend_id", fmt.Sprintf("%d", p.FriendID))
	}
	if p.DatedAfter != "" {
		params.Set("dated_after", p.DatedAfter)
	}
	if p.DatedBefore != "" {
		params.Set("dated_before", p.DatedBefore)
	}
	if p.UpdatedAfter != "" {
		params.Set("updated_after", p.UpdatedAfter)
	}
	if p.UpdatedBefore != "" {
		params.Set("updated_before", p.UpdatedBefore)
	}
	if p.Limit > 0 {
		params.Set("limit", fmt.Sprintf("%d", p.Limit))
	}
	if p.Offset > 0 {
		params.Set("offset", fmt.Sprintf("%d", p.Offset))
	}
	if p.Visible != nil {
		params.Set("visible", fmt.Sprintf("%t", *p.Visible))
	}
	data, err := c.get("/get_expenses", params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Expenses []Expense `json:"expenses"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Expenses, nil
}

// GetExpense returns a single expense by ID.
func (c *Client) GetExpense(id int64) (*Expense, error) {
	data, err := c.get(fmt.Sprintf("/get_expense/%d", id), nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Expense *Expense `json:"expense"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if resp.Expense == nil {
		return nil, fmt.Errorf("expense not found: %d", id)
	}
	return resp.Expense, nil
}

// CreateExpenseParams holds parameters for creating an expense.
type CreateExpenseParams struct {
	Description  string
	Cost         string
	CurrencyCode string
	GroupID      int64
	SplitEqually bool
	Date         string // ISO 8601 date or datetime; empty = today on the server
	Details      string // free-form notes
	CategoryID   int    // Splitwise subcategory ID (0 = default)
	// Shares is used for non-even splits. Each entry maps a user to their
	// paid and owed share of the expense.
	Shares []ShareParam
}

type ShareParam struct {
	UserID    int64
	PaidShare string
	OwedShare string
}

// encodeShares encodes a list of per-user shares using Splitwise's
// `users__N__field` naming convention into the given url.Values.
func encodeShares(params url.Values, shares []ShareParam) {
	for i, s := range shares {
		prefix := fmt.Sprintf("users__%d__", i)
		params.Set(prefix+"user_id", fmt.Sprintf("%d", s.UserID))
		params.Set(prefix+"paid_share", s.PaidShare)
		params.Set(prefix+"owed_share", s.OwedShare)
	}
}

// encodeCreateExpense builds the form body shared by CreateExpense and
// CreatePayment. `payment` controls the payment=true flag.
func encodeCreateExpense(p CreateExpenseParams, payment bool) url.Values {
	params := url.Values{}
	params.Set("description", p.Description)
	params.Set("cost", p.Cost)
	if payment {
		params.Set("payment", "true")
	}
	if p.CurrencyCode != "" {
		params.Set("currency_code", p.CurrencyCode)
	}
	if p.Date != "" {
		params.Set("date", p.Date)
	}
	if p.Details != "" {
		params.Set("details", p.Details)
	}
	if p.CategoryID > 0 {
		params.Set("category_id", fmt.Sprintf("%d", p.CategoryID))
	}

	switch {
	case p.SplitEqually:
		params.Set("group_id", fmt.Sprintf("%d", p.GroupID))
		params.Set("split_equally", "true")
	case len(p.Shares) > 0:
		if p.GroupID > 0 {
			params.Set("group_id", fmt.Sprintf("%d", p.GroupID))
		} else {
			params.Set("group_id", "0")
		}
		encodeShares(params, p.Shares)
	}
	return params
}

// CreateExpense creates a new expense.
func (c *Client) CreateExpense(p CreateExpenseParams) (*Expense, error) {
	params := encodeCreateExpense(p, false)
	return c.postExpense("/create_expense", params, "expense creation failed")
}

// CreatePayment records a settlement (payment) between users. Description
// defaults to "Payment" when not explicitly set.
func (c *Client) CreatePayment(p CreateExpenseParams) (*Expense, error) {
	if p.Description == "" {
		p.Description = "Payment"
	}
	params := encodeCreateExpense(p, true)
	return c.postExpense("/create_expense", params, "settlement failed")
}

// postExpense POSTs a create_expense form and parses the shared response envelope.
func (c *Client) postExpense(path string, params url.Values, errPrefix string) (*Expense, error) {
	data, err := c.post(path, params)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Expenses []Expense         `json:"expenses"`
		Errors   map[string]any    `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if msg := formatAPIErrors(resp.Errors); msg != "" {
		return nil, fmt.Errorf("%s: %s", errPrefix, msg)
	}
	if len(resp.Expenses) == 0 {
		return nil, fmt.Errorf("no expense returned")
	}
	return &resp.Expenses[0], nil
}

// DeleteExpense deletes an expense by ID.
func (c *Client) DeleteExpense(id int64) error {
	data, err := c.post(fmt.Sprintf("/delete_expense/%d", id), nil)
	if err != nil {
		return err
	}
	var resp struct {
		Success bool           `json:"success"`
		Errors  map[string]any `json:"errors"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	if !resp.Success {
		if msg := formatAPIErrors(resp.Errors); msg != "" {
			return fmt.Errorf("delete failed: %s", msg)
		}
		return fmt.Errorf("delete failed")
	}
	return nil
}

// formatAPIErrors flattens Splitwise's {"errors": {"field": ["msg", ...]}}
// envelope (also handles flat string maps and scalar values) into a single
// human-readable string. Returns "" if there are no errors.
func formatAPIErrors(errs map[string]any) string {
	if len(errs) == 0 {
		return ""
	}
	// Sort keys for stable output.
	keys := make([]string, 0, len(errs))
	for k := range errs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		for _, msg := range flattenErrorValue(errs[k]) {
			if k == "base" {
				parts = append(parts, msg)
			} else {
				parts = append(parts, fmt.Sprintf("%s: %s", k, msg))
			}
		}
	}
	return strings.Join(parts, "; ")
}

func flattenErrorValue(v any) []string {
	switch x := v.(type) {
	case string:
		if x == "" {
			return nil
		}
		return []string{x}
	case []any:
		var out []string
		for _, item := range x {
			out = append(out, flattenErrorValue(item)...)
		}
		return out
	case map[string]any:
		var out []string
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for _, inner := range flattenErrorValue(x[k]) {
				out = append(out, fmt.Sprintf("%s: %s", k, inner))
			}
		}
		return out
	case nil:
		return nil
	default:
		return []string{fmt.Sprintf("%v", x)}
	}
}

// Category is a Splitwise expense category (e.g. "Groceries").
// ParentCategories contain subcategories — only subcategories can be used
// as category_id on create_expense.
type ParentCategory struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	Icon          string     `json:"icon,omitempty"`
	Subcategories []Category `json:"subcategories,omitempty"`
}

// GetCategories returns the full category tree.
func (c *Client) GetCategories() ([]ParentCategory, error) {
	data, err := c.get("/get_categories", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Categories []ParentCategory `json:"categories"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Categories, nil
}

// Currency is a Splitwise currency descriptor.
type Currency struct {
	CurrencyCode string `json:"currency_code"`
	Unit         string `json:"unit"`
}

// GetCurrencies returns the list of supported currencies.
func (c *Client) GetCurrencies() ([]Currency, error) {
	data, err := c.get("/get_currencies", nil)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Currencies []Currency `json:"currencies"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	return resp.Currencies, nil
}

// ResolveGroupByName finds a group ID by name (case-insensitive partial match).
func (c *Client) ResolveGroupByName(name string) (*Group, error) {
	groups, err := c.GetGroups()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for i := range groups {
		if strings.ToLower(groups[i].Name) == lower {
			return &groups[i], nil
		}
	}
	// Partial match fallback.
	for i := range groups {
		if strings.Contains(strings.ToLower(groups[i].Name), lower) {
			return &groups[i], nil
		}
	}
	return nil, fmt.Errorf("group not found: %s", name)
}

// ResolveFriendByName finds a friend by name (case-insensitive).
func (c *Client) ResolveFriendByName(name string) (*Friend, error) {
	friends, err := c.GetFriends()
	if err != nil {
		return nil, err
	}
	lower := strings.ToLower(name)
	for i := range friends {
		fullName := strings.ToLower(friends[i].FirstName + " " + friends[i].LastName)
		if fullName == lower || strings.ToLower(friends[i].FirstName) == lower {
			return &friends[i], nil
		}
	}
	// Partial match.
	for i := range friends {
		fullName := strings.ToLower(friends[i].FirstName + " " + friends[i].LastName)
		if strings.Contains(fullName, lower) {
			return &friends[i], nil
		}
	}
	return nil, fmt.Errorf("friend not found: %s", name)
}
