package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/example/splitwise-cli/internal/api"
	"github.com/example/splitwise-cli/internal/config"
	"github.com/example/splitwise-cli/internal/output"
	"github.com/spf13/cobra"
)

var expensesCmd = &cobra.Command{
	Use:   "expenses",
	Short: "Manage expenses",
}

var expensesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List expenses",
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		groupName, _ := cmd.Flags().GetString("group")
		limit, _ := cmd.Flags().GetInt("limit")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		showAll, _ := cmd.Flags().GetBool("all")

		// Resolve group if specified.
		if groupName == "" {
			cfg, _ := config.Load()
			if cfg != nil {
				groupName = cfg.DefaultGroup
			}
		}

		p := api.GetExpensesParams{
			Limit:       limit,
			DatedAfter:  after,
			DatedBefore: before,
		}

		if groupName != "" {
			group, err := client.ResolveGroupByName(groupName)
			if err != nil {
				output.Die("%v", err)
			}
			p.GroupID = group.ID
		}

		expenses, err := client.GetExpenses(p)
		if err != nil {
			output.Die("%v", err)
		}

		// /get_expenses returns soft-deleted rows; hide them unless --all
		// is set. (The Splitwise API has no server-side filter for this.)
		if !showAll {
			filtered := expenses[:0]
			for _, e := range expenses {
				if e.DeletedAt == nil {
					filtered = append(filtered, e)
				}
			}
			expenses = filtered
		}

		if jsonOut {
			output.JSON(expenses)
			return
		}

		if quiet {
			for _, e := range expenses {
				fmt.Printf("%d\t%s\t%s\n", e.ID, e.Cost, e.CurrencyCode)
			}
			return
		}

		var rows [][]string
		for _, e := range expenses {
			date := e.Date
			if t, err := time.Parse(time.RFC3339, e.Date); err == nil {
				date = t.Format("2006-01-02")
			}
			desc := e.Description
			if e.Payment {
				desc = output.Faint.Sprint("💸 " + desc)
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", e.ID),
				date,
				desc,
				fmt.Sprintf("%s %s", e.Cost, e.CurrencyCode),
			})
		}
		output.Table([]string{"ID", "Date", "Description", "Amount"}, rows)
	},
}

var expensesCreateCmd = &cobra.Command{
	Use:   `create "description" AMOUNT`,
	Short: "Create a new expense",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		description := args[0]
		cost := args[1]
		groupName, _ := cmd.Flags().GetString("group")
		split, _ := cmd.Flags().GetString("split")
		currency, _ := cmd.Flags().GetString("currency")
		paidBy, _ := cmd.Flags().GetString("paid-by")
		date, _ := cmd.Flags().GetString("date")
		details, _ := cmd.Flags().GetString("details")

		// Resolve defaults.
		cfg, _ := config.Load()
		if groupName == "" && cfg != nil {
			groupName = cfg.DefaultGroup
		}
		if currency == "" && cfg != nil {
			currency = cfg.DefaultCurrency
		}

		if groupName == "" {
			output.Die("group is required — pass --group or set default_group")
		}

		group, err := client.ResolveGroupByName(groupName)
		if err != nil {
			output.Die("%v", err)
		}

		p := api.CreateExpenseParams{
			Description:  description,
			Cost:         cost,
			CurrencyCode: currency,
			GroupID:      group.ID,
			Date:         date,
			Details:      details,
		}

		switch {
		case split == "" || split == "even":
			if paidBy == "" {
				// Default case: split equally, authenticated user pays.
				p.SplitEqually = true
			} else {
				// Custom payer with even split: the API's split_equally=true
				// always makes the authenticated user the payer, so build
				// per-member shares manually.
				payerID, err := resolveMemberInGroup(group, paidBy)
				if err != nil {
					output.Die("%v", err)
				}
				shares, err := buildEvenShares(group.Members, cost, payerID)
				if err != nil {
					output.Die("%v", err)
				}
				p.Shares = shares
			}
		case strings.HasPrefix(split, "exact:"):
			payerID, err := resolvePayer(client, group, paidBy)
			if err != nil {
				output.Die("%v", err)
			}
			shares, err := buildExactShares(group.Members, cost, payerID, split[len("exact:"):])
			if err != nil {
				output.Die("%v", err)
			}
			p.Shares = shares
		case split == "exact":
			output.Die("exact split requires amounts — use format: exact:Name:Amount,Name:Amount")
		default:
			output.Die("unknown split type: %s", split)
		}

		expense, err := client.CreateExpense(p)
		if err != nil {
			output.Die("%v", err)
		}

		if jsonOut {
			output.JSON(expense)
			return
		}

		if quiet {
			fmt.Println(expense.ID)
			return
		}

		output.Green.Printf("✓ Created expense #%d\n", expense.ID)
		fmt.Printf("  %s — %s %s\n", expense.Description, expense.Cost, expense.CurrencyCode)
	},
}

var expensesShowCmd = &cobra.Command{
	Use:   "show <id>",
	Short: "Show full details for a single expense",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			output.Die("invalid expense ID: %s", args[0])
		}

		expense, err := client.GetExpense(id)
		if err != nil {
			output.Die("%v", err)
		}

		if jsonOut {
			output.JSON(expense)
			return
		}

		date := expense.Date
		if t, err := time.Parse(time.RFC3339, expense.Date); err == nil {
			date = t.Format("2006-01-02")
		}

		output.Bold.Printf("%s\n", expense.Description)
		fmt.Printf("  ID:       %d\n", expense.ID)
		fmt.Printf("  Date:     %s\n", date)
		fmt.Printf("  Amount:   %s %s\n", expense.Cost, expense.CurrencyCode)
		if expense.Category != nil {
			fmt.Printf("  Category: %s\n", expense.Category.Name)
		}
		if expense.Details != nil && *expense.Details != "" {
			fmt.Printf("  Notes:    %s\n", *expense.Details)
		}
		if expense.Payment {
			fmt.Println("  Type:     Payment / settlement")
		}
		if expense.DeletedAt != nil {
			output.Faint.Printf("  (deleted at %s)\n", *expense.DeletedAt)
		}
		if len(expense.Users) > 0 {
			fmt.Println()
			output.Bold.Println("Shares:")
			for _, s := range expense.Users {
				name := "?"
				if s.User != nil {
					name = strings.TrimSpace(s.User.FirstName + " " + s.User.LastName)
				}
				fmt.Printf("  %-20s paid %s, owes %s\n", name, s.PaidShare, s.OwedShare)
			}
		}
	},
}

var expensesDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete an expense",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			output.Die("invalid expense ID: %s", args[0])
		}

		if err := client.DeleteExpense(id); err != nil {
			output.Die("%v", err)
		}

		if quiet {
			return
		}

		output.Green.Printf("✓ Deleted expense #%d\n", id)
	},
}

var expensesRestoreCmd = &cobra.Command{
	Use:   "restore <id>",
	Short: "Restore a previously deleted expense",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			output.Die("invalid expense ID: %s", args[0])
		}

		if err := client.UndeleteExpense(id); err != nil {
			output.Die("%v", err)
		}

		if quiet {
			return
		}

		output.Green.Printf("✓ Restored expense #%d\n", id)
	},
}

// resolveMemberInGroup returns the ID of the group member whose first name
// or full name matches `name` (case-insensitive).
func resolveMemberInGroup(group *api.Group, name string) (int64, error) {
	lower := strings.ToLower(strings.TrimSpace(name))
	for _, m := range group.Members {
		first := strings.ToLower(m.FirstName)
		full := strings.ToLower(strings.TrimSpace(m.FirstName + " " + m.LastName))
		if first == lower || full == lower {
			return m.ID, nil
		}
	}
	return 0, fmt.Errorf("user not found in group: %s", name)
}

// resolvePayer returns the payer's user ID: the user named by `paidBy` if set,
// otherwise the currently authenticated user.
func resolvePayer(client *api.Client, group *api.Group, paidBy string) (int64, error) {
	if paidBy != "" {
		return resolveMemberInGroup(group, paidBy)
	}
	me, err := client.GetCurrentUser()
	if err != nil {
		return 0, fmt.Errorf("failed to get current user: %w", err)
	}
	return me.ID, nil
}

// buildEvenShares splits `cost` evenly among `members` with `payerID` paying
// the full amount. Amounts are computed in cents so shares sum exactly to the
// cost; any rounding remainder is distributed one cent at a time across
// members (ordered by ID for deterministic output).
func buildEvenShares(members []api.GroupMember, cost string, payerID int64) ([]api.ShareParam, error) {
	if len(members) == 0 {
		return nil, fmt.Errorf("group has no members")
	}
	costFloat, err := strconv.ParseFloat(cost, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid cost: %s", cost)
	}
	costCents := int64(math.Round(costFloat * 100))
	n := int64(len(members))
	base := costCents / n
	remainder := costCents - base*n

	payerSeen := false
	shares := make([]api.ShareParam, 0, len(members))
	for i, m := range members {
		if m.ID == payerID {
			payerSeen = true
		}
		owedCents := base
		if int64(i) < remainder {
			owedCents++
		}
		paid := "0.00"
		if m.ID == payerID {
			paid = fmt.Sprintf("%.2f", float64(costCents)/100)
		}
		shares = append(shares, api.ShareParam{
			UserID:    m.ID,
			PaidShare: paid,
			OwedShare: fmt.Sprintf("%.2f", float64(owedCents)/100),
		})
	}
	if !payerSeen {
		return nil, fmt.Errorf("payer is not a member of the group")
	}
	return shares, nil
}

// buildExactShares parses "Name:Amount,Name:Amount" and builds shares for
// every group member. `payerID` pays the full cost; members not mentioned in
// the spec owe 0. The sum of owed amounts must equal `cost`.
func buildExactShares(members []api.GroupMember, cost string, payerID int64, spec string) ([]api.ShareParam, error) {
	owedMap := make(map[string]string)
	var owedTotal float64
	for _, pair := range strings.Split(spec, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid split format: %s (expected Name:Amount)", pair)
		}
		name := strings.TrimSpace(parts[0])
		amount := strings.TrimSpace(parts[1])
		amt, err := strconv.ParseFloat(amount, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid amount for %s: %s", name, amount)
		}
		owedMap[strings.ToLower(name)] = fmt.Sprintf("%.2f", amt)
		owedTotal += amt
	}

	costFloat, err := strconv.ParseFloat(cost, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid cost: %s", cost)
	}
	if fmt.Sprintf("%.2f", owedTotal) != fmt.Sprintf("%.2f", costFloat) {
		return nil, fmt.Errorf("split amounts (%.2f) don't add up to total (%.2f)", owedTotal, costFloat)
	}

	payerSeen := false
	shares := make([]api.ShareParam, 0, len(members))
	for _, m := range members {
		if m.ID == payerID {
			payerSeen = true
		}
		paid := "0.00"
		if m.ID == payerID {
			paid = cost
		}
		owed := "0.00"
		lowerFirst := strings.ToLower(m.FirstName)
		lowerFull := strings.ToLower(strings.TrimSpace(m.FirstName + " " + m.LastName))
		if amt, ok := owedMap[lowerFirst]; ok {
			owed = amt
		} else if amt, ok := owedMap[lowerFull]; ok {
			owed = amt
		}
		shares = append(shares, api.ShareParam{
			UserID:    m.ID,
			PaidShare: paid,
			OwedShare: owed,
		})
	}
	if !payerSeen {
		return nil, fmt.Errorf("payer is not a member of the group")
	}
	return shares, nil
}

func init() {
	expensesListCmd.Flags().StringP("group", "g", "", "Filter by group name")
	expensesListCmd.Flags().IntP("limit", "l", 20, "Maximum number of expenses")
	expensesListCmd.Flags().String("after", "", "Only expenses after this date (YYYY-MM-DD)")
	expensesListCmd.Flags().String("before", "", "Only expenses before this date (YYYY-MM-DD)")
	expensesListCmd.Flags().Bool("all", false, "Include deleted expenses")

	expensesCreateCmd.Flags().StringP("group", "g", "", "Group to add expense to")
	expensesCreateCmd.Flags().String("split", "even", `Split type: even, or exact:Name:Amount,Name:Amount (e.g. "exact:MemberA:60,MemberB:40")`)
	expensesCreateCmd.Flags().String("paid-by", "", "Who paid (name, defaults to you)")
	expensesCreateCmd.Flags().StringP("currency", "c", "", "Currency code (e.g. USD)")
	expensesCreateCmd.Flags().String("date", "", "Expense date (YYYY-MM-DD, defaults to today)")
	expensesCreateCmd.Flags().String("details", "", "Notes attached to the expense")

	expensesCmd.AddCommand(expensesListCmd)
	expensesCmd.AddCommand(expensesCreateCmd)
	expensesCmd.AddCommand(expensesDeleteCmd)
	expensesCmd.AddCommand(expensesRestoreCmd)
	expensesCmd.AddCommand(expensesShowCmd)
	rootCmd.AddCommand(expensesCmd)
}
