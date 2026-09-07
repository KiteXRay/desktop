package bridge

// CheckRunningProcesses checks running system processes against active rules
// and returns a map of ruleID -> count of matching running processes.
func CheckRunningProcesses(rules []BridgeRule, groups ...[]BridgeGroup) map[string]int {
	results := make(map[string]int)
	for _, r := range rules {
		results[r.ID] = 0
	}

	procs, err := GetRunningProcesses()
	if err != nil {
		return results
	}

	var disabledGroups map[string]bool
	if len(groups) > 0 {
		for _, g := range groups[0] {
			if !g.Enabled {
				if disabledGroups == nil {
					disabledGroups = make(map[string]bool)
				}
				disabledGroups[g.ID] = true
			}
		}
	}

	for _, r := range rules {
		if !r.Enabled {
			continue
		}
		if r.GroupID != "" && disabledGroups != nil && disabledGroups[r.GroupID] {
			continue
		}
		count := 0
		for _, proc := range procs {
			if r.Matches(proc) {
				count++
			}
		}
		results[r.ID] = count
	}

	return results
}
