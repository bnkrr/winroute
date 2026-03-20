package aliascheck

import "fmt"

// ValidateUniqueAlias rejects aliases that map to multiple interfaces.
func ValidateUniqueAlias(alias string, count int) error {
	if count <= 1 {
		return nil
	}

	return fmt.Errorf(
		"interface alias %q matches %d interfaces; use interface index instead",
		alias,
		count,
	)
}
