package aliascheck

import "testing"

func TestValidateUniqueAlias(t *testing.T) {
	if err := ValidateUniqueAlias("Ethernet", 1); err != nil {
		t.Fatalf("expected unique alias to pass, got %v", err)
	}

	err := ValidateUniqueAlias("Ethernet", 2)
	if err == nil {
		t.Fatal("expected duplicate alias to fail")
	}
	if got := err.Error(); got == "" {
		t.Fatal("expected duplicate alias error message")
	}
}
