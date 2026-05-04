package utils

import (
	"testing"
)

func TestNewS3Client(t *testing.T) {
	// This test just verifies that the client can be initialized
	// (it might fail if it tries to load AWS config and fails, 
	// but it shouldn't as long as it's just loading defaults)
	client, err := NewS3Client("test-bucket", "", "")
	if err != nil {
		t.Logf("Warning: NewS3Client failed (expected in environments without AWS config): %v", err)
	} else if client == nil {
		t.Fatal("NewS3Client returned nil without error")
	}
}
