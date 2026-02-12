package vault

import (
	"testing"
)

func TestSetupIdentity(t *testing.T) {
	t.Run("Decoupled Gemini Link", func(t *testing.T) {
		// Just call it to see if it doesn't crash (though it might fail if sudo is not available)
		// For now, just prove we can reference it.
		_ = SetupIdentity
	})
}
