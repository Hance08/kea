package cmd

import (
	"fmt"

	"github.com/hance08/kea/internal/store"
	"github.com/pterm/pterm"
)

func displayAccountsTree(accounts []*store.Account, showBalance bool) {
	childrenMap := make(map[int64][]*store.Account)
	accountMap := make(map[int64]*store.Account)
	var roots []*store.Account

	// Populate maps
	for _, acc := range accounts {
		accountMap[acc.ID] = acc
	}

	// Build hierarchy
	for _, acc := range accounts {
		isRoot := false
		if acc.ParentID == nil {
			isRoot = true
		} else {
			if _, parentExists := accountMap[*acc.ParentID]; !parentExists {
				isRoot = true
			}
		}

		if isRoot {
			roots = append(roots, acc)
		} else {
			childrenMap[*acc.ParentID] = append(childrenMap[*acc.ParentID], acc)
		}
	}

	// Recursive builder
	var buildNode func(acc *store.Account) pterm.TreeNode
	buildNode = func(acc *store.Account) pterm.TreeNode {
		displayText := acc.Name
		if showBalance {
			balance, _ := logic.GetAccountBalanceFormatted(acc.ID)
			displayText += fmt.Sprintf(" | %s", pterm.Green(balance))
		}

		node := pterm.TreeNode{
			Text: displayText,
		}

		for _, child := range childrenMap[acc.ID] {
			node.Children = append(node.Children, buildNode(child))
		}
		return node
	}

	var treeData []pterm.TreeNode
	for _, root := range roots {
		treeData = append(treeData, buildNode(root))
	}

	pterm.DefaultSection.Println("Account Tree")
	pterm.DefaultTree.WithRoot(pterm.TreeNode{Text: "Accounts", Children: treeData}).Render()
	pterm.Println()
	pterm.Info.Printf("Total: %d accounts\n", len(accounts))
}
