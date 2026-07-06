package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestBuildProducesRunningBinary verifies the binary builds and responds to --help.
func TestBuildProducesRunningBinary(t *testing.T) {
	binaryPath := filepath.Join("..", "..", "yg")

	buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
	output, err := buildCmd.CombinedOutput()
	if !assert.NoError(t, err, "go build failed: %s", string(output)) {
		return
	}
	t.Cleanup(func() { os.Remove(binaryPath) })

	helpCmd := exec.Command(binaryPath, "--help")
	err = helpCmd.Run()
	assert.NoError(t, err, "yg --help should exit 0")
}
