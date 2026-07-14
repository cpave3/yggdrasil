package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createIndexedWorktree(t *testing.T, binary, repoDir, branch string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, ".yggdrasil.toml"), []byte(`
[general]
path_template = "../{repo}.{branch}"
`), 0644))

	cmd := exec.Command(binary, "new", branch)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg new failed: %s", string(out))
	return filepath.Join(filepath.Dir(repoDir), filepath.Base(repoDir)+"."+branch)
}

func TestCD_PrintsIndexedWorktreePath(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)
	worktreePath := createIndexedWorktree(t, binary, repoDir, "feature-cd")

	cmd := exec.Command(binary, "cd", "1")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg cd failed: %s", string(out))
	assert.Equal(t, worktreePath, strings.TrimSpace(string(out)))
}

func TestCD_RejectsInvalidIndices(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)

	for _, index := range []string{"abc", "-1", "1"} {
		t.Run(index, func(t *testing.T) {
			cmd := exec.Command(binary, "cd", index)
			cmd.Dir = repoDir
			out, err := cmd.CombinedOutput()
			require.Error(t, err)
			if index == "1" {
				assert.Contains(t, string(out), "out of range")
			} else {
				assert.Contains(t, string(out), "non-negative integer")
			}
		})
	}
}

func TestShellIntegration_CDChangesWorkingDirectory(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)
	worktreePath := createIndexedWorktree(t, binary, repoDir, "feature-shell")
	binaryDir := filepath.Dir(binary)

	script := fmt.Sprintf(`eval "$(command yg shell-init)"
yg cd 1
printf '%%s' "$PWD"
`)
	cmd := exec.Command("bash", "-c", script)
	cmd.Dir = repoDir
	cmd.Env = append(os.Environ(), "PATH="+binaryDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "shell integration failed: %s", string(out))
	assert.Equal(t, worktreePath, string(out))
}

func TestList_IndicesMatchCDResolution(t *testing.T) {
	binary := buildBinary(t)
	repoDir := initTestRepo(t)
	worktreePath := createIndexedWorktree(t, binary, repoDir, "feature-index")

	cmd := exec.Command(binary, "ls")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "yg ls failed: %s", string(out))

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	require.Len(t, lines, 3)
	assert.Equal(t, "INDEX", strings.Fields(lines[0])[0])
	assert.Equal(t, "0", strings.Fields(lines[1])[0])
	assert.Equal(t, "1", strings.Fields(lines[2])[0])
	assert.Contains(t, lines[2], worktreePath)

	index, err := strconv.Atoi(strings.Fields(lines[2])[0])
	require.NoError(t, err)
	cd := exec.Command(binary, "cd", strconv.Itoa(index))
	cd.Dir = repoDir
	resolved, err := cd.CombinedOutput()
	require.NoError(t, err, "yg cd failed: %s", string(resolved))
	assert.Equal(t, worktreePath, strings.TrimSpace(string(resolved)))
}
