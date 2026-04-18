package utils

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsDockerAvailable checks if Docker is installed and running
func IsDockerAvailable() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker is not available or not running: %w", err)
	}
	return nil
}

// EnsureDir creates a directory if it does not exist
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	return nil
}

// CopyDir recursively copies a directory tree, attempting to preserve permissions.
// Source directory must exist, destination directory must not exist.
// Any absolute paths in excludePaths will be skipped during the copy.
func CopyDir(src string, dst string, excludePaths ...string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip excluded paths to prevent infinite recursion when dst is inside src
		skip := false
		for _, excl := range excludePaths {
			if srcPath == excl {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		if entry.IsDir() {
			if err = CopyDir(srcPath, dstPath, excludePaths...); err != nil {
				return err
			}
		} else {
			if err = CopyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyFile copies a single file from src to dst.
func CopyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	if _, err = io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// PrintBoxedMessage prints a message in a box with borders
func PrintBoxedMessage(lines []string) {
	if len(lines) == 0 {
		return
	}

	maxLen := 0
	for _, line := range lines {
		if len(line) > maxLen {
			maxLen = len(line)
		}
	}

	width := maxLen + 2

	fmt.Println("╔" + strings.Repeat("═", width) + "╗")
	for _, line := range lines {
		padding := width - len(line) - 1
		if padding < 0 {
			padding = 0
		}
		fmt.Printf("║ %s%s║\n", line, strings.Repeat(" ", padding))
	}
	fmt.Println("╚" + strings.Repeat("═", width) + "╝")
}
