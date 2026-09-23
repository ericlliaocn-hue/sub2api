package service

import "strings"

// Daily-throwaway (日抛) groups must never land on the long-lived 20x reserve
// or the luna-only cars. Membership mistakes have repeatedly leaked them onto
// those accounts; this denylist is the last gate.
var dailyThrowawayForbiddenEmails = []string{
	"zbj9786@gmail.com",
	"beilegedong@gmail.com",
	"cuadrasdeshon335@gmail.com",
}

func IsDailyThrowawayGroup(group *Group) bool {
	if group == nil {
		return false
	}
	return strings.Contains(group.Name, "日抛")
}

func IsDailyThrowawayForbiddenAccount(account *Account) bool {
	if account == nil {
		return false
	}
	blob := strings.ToLower(strings.TrimSpace(account.Name))
	if notes := account.Notes; notes != nil {
		blob += " " + strings.ToLower(strings.TrimSpace(*notes))
	}
	for _, email := range dailyThrowawayForbiddenEmails {
		if strings.Contains(blob, email) {
			return true
		}
	}
	return false
}

func filterDailyThrowawayForbiddenAccounts(group *Group, accounts []*Account) []*Account {
	if !IsDailyThrowawayGroup(group) || len(accounts) == 0 {
		return accounts
	}
	out := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if IsDailyThrowawayForbiddenAccount(account) {
			continue
		}
		out = append(out, account)
	}
	return out
}
