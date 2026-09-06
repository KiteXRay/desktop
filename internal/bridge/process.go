package bridge

// CheckRunningProcesses checks running system processes against active rules
// and returns a map of ruleID -> count of matching running processes.
func CheckRunningProcesses(rules []BridgeRule) map[string]int {
	results := make(map[string]int)
	for _, r := range rules {
		results[r.ID] = 0
	}

	procs, err := GetRunningProcesses()
	if err != nil {
		return results
	}

	for _, r := range rules {
		if !r.Enabled {
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
