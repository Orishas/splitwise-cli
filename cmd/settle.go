package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/example/splitwise-cli/internal/api"
	"github.com/example/splitwise-cli/internal/config"
	"github.com/example/splitwise-cli/internal/output"
	"github.com/spf13/cobra"
)

var settleCmd = &cobra.Command{
	Use:   "settle <friend-name|id>",
	Short: "Record a settlement payment",
	Long: `Record a settlement payment with a friend.

The settlement amount is determined automatically from the outstanding
balance. Use --group to settle within a specific group context.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := api.New()
		if err != nil {
			output.Die("%v", err)
		}

		groupName, _ := cmd.Flags().GetString("group")
		if groupName == "" {
			cfg, _ := config.Load()
			if cfg != nil {
				groupName = cfg.DefaultGroup
			}
		}

		// Resolve friend.
		var friend *api.Friend
		if id, err := strconv.ParseInt(args[0], 10, 64); err == nil {
			friends, err := client.GetFriends()
			if err != nil {
				output.Die("%v", err)
			}
			for i := range friends {
				if friends[i].ID == id {
					friend = &friends[i]
					break
				}
			}
			if friend == nil {
				output.Die("friend not found with ID: %d", id)
			}
		} else {
			friend, err = client.ResolveFriendByName(args[0])
			if err != nil {
				output.Die("%v", err)
			}
		}

		me, err := client.GetCurrentUser()
		if err != nil {
			output.Die("failed to get current user: %v", err)
		}

		// Collect all non-zero balances (Splitwise may track balances per currency).
		type pendingBal struct {
			amount float64
			code   string
		}
		var pending []pendingBal
		for _, b := range friend.Balance {
			amt, err := strconv.ParseFloat(b.Amount, 64)
			if err != nil {
				output.Die("invalid balance amount: %s", b.Amount)
			}
			if amt != 0 {
				pending = append(pending, pendingBal{amount: amt, code: b.CurrencyCode})
			}
		}
		if len(pending) == 0 {
			fmt.Printf("Already settled up with %s %s! 🎉\n", friend.FirstName, friend.LastName)
			return
		}

		var groupID int64
		if groupName != "" {
			group, err := client.ResolveGroupByName(groupName)
			if err != nil {
				output.Die("%v", err)
			}
			groupID = group.ID
		}

		friendName := strings.TrimSpace(friend.FirstName + " " + friend.LastName)
		var recorded []*api.Expense
		for _, bal := range pending {
			absAmount := fmt.Sprintf("%.2f", math.Abs(bal.amount))

			p := api.CreateExpenseParams{
				Cost:         absAmount,
				CurrencyCode: bal.code,
				GroupID:      groupID,
			}
			if bal.amount > 0 {
				// Friend owes me → friend pays me.
				p.Shares = []api.ShareParam{
					{UserID: friend.ID, PaidShare: absAmount, OwedShare: "0.00"},
					{UserID: me.ID, PaidShare: "0.00", OwedShare: absAmount},
				}
			} else {
				// I owe friend → I pay friend.
				p.Shares = []api.ShareParam{
					{UserID: me.ID, PaidShare: absAmount, OwedShare: "0.00"},
					{UserID: friend.ID, PaidShare: "0.00", OwedShare: absAmount},
				}
			}

			expense, err := client.CreatePayment(p)
			if err != nil {
				output.Die("%v", err)
			}
			recorded = append(recorded, expense)

			if jsonOut || quiet {
				continue
			}
			if bal.amount > 0 {
				output.Green.Printf("✓ Recorded settlement: %s paid you %s %s\n", friendName, absAmount, bal.code)
			} else {
				output.Green.Printf("✓ Recorded settlement: you paid %s %s %s\n", friendName, absAmount, bal.code)
			}
		}

		if jsonOut {
			output.JSON(recorded)
			return
		}
		if quiet {
			for _, e := range recorded {
				fmt.Println(e.ID)
			}
			return
		}
	},
}

func init() {
	settleCmd.Flags().StringP("group", "g", "", "Settle within a specific group")
	rootCmd.AddCommand(settleCmd)
}
