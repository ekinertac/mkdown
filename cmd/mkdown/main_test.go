package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainIntegration(t *testing.T) {
	// Build the binary for testing
	tmpBinary := filepath.Join(t.TempDir(), "mkdown-test")
	cmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	tests := []struct {
		name       string
		args       []string
		input      string
		wantErr    bool
		checkStdout string
	}{
		{
			name:       "version flag",
			args:       []string{"--version"},
			wantErr:    false,
			checkStdout: "mkdown v",
		},
		{
			name:       "help flag",
			args:       []string{"--help"},
			wantErr:    false,
			checkStdout: "Usage:",
		},
		{
			name:    "no arguments",
			args:    []string{},
			wantErr: true,
		},
		{
			name:    "invalid theme",
			args:    []string{"test.md", "--theme", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command(tmpBinary, tt.args...)
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v\nOutput: %s", err, output)
			}

			if tt.checkStdout != "" && !strings.Contains(string(output), tt.checkStdout) {
				t.Errorf("expected output to contain %q, got: %s", tt.checkStdout, output)
			}
		})
	}
}

func TestMainConversion(t *testing.T) {
	// Build the binary
	tmpBinary := filepath.Join(t.TempDir(), "mkdown-test")
	cmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "test.md")
	
	content := `---
title: Test Doc
---

# Hello

This is a test.`

	if err := os.WriteFile(inputPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test default output
	t.Run("default output", func(t *testing.T) {
		cmd := exec.Command(tmpBinary, inputPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("conversion failed: %v\nOutput: %s", err, output)
		}

		expectedOutput := filepath.Join(tmpDir, "test.html")
		if _, err := os.Stat(expectedOutput); os.IsNotExist(err) {
			t.Error("expected output file not created")
		}

		// Check output message
		if !strings.Contains(string(output), "✓ Generated:") {
			t.Error("success message not printed")
		}
	})

	// Test custom output
	t.Run("custom output", func(t *testing.T) {
		customOutput := filepath.Join(tmpDir, "custom.html")
		cmd := exec.Command(tmpBinary, inputPath, "-o", customOutput)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("conversion failed: %v\nOutput: %s", err, output)
		}

		if _, err := os.Stat(customOutput); os.IsNotExist(err) {
			t.Error("custom output file not created")
		}
	})

	// Test theme selection
	t.Run("light theme", func(t *testing.T) {
		lightOutput := filepath.Join(tmpDir, "light.html")
		cmd := exec.Command(tmpBinary, inputPath, "-t", "light", "-o", lightOutput)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("conversion failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(string(output), "theme: light") {
			t.Error("theme not shown in output")
		}

		// Check light theme colors in output
		htmlContent, err := os.ReadFile(lightOutput)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(htmlContent), "#ffffff") {
			t.Error("light theme colors not found in output")
		}
	})
}

func TestMainFileValidation(t *testing.T) {
	tmpBinary := filepath.Join(t.TempDir(), "mkdown-test")
	cmd := exec.Command("go", "build", "-o", tmpBinary, ".")
	cmd.Dir = "."
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to build binary: %v", err)
	}

	tests := []struct {
		name     string
		filename string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "nonexistent file",
			filename: "nonexistent.md",
			wantErr:  true,
			errMsg:   "not found",
		},
		{
			name:     "non-markdown file",
			filename: "test.txt",
			wantErr:  true,
			errMsg:   "must be a markdown file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create txt file for the non-markdown test
			if strings.HasSuffix(tt.filename, ".txt") {
				tmpDir := t.TempDir()
				txtFile := filepath.Join(tmpDir, tt.filename)
				os.WriteFile(txtFile, []byte("test"), 0644)
				tt.filename = txtFile
			}

			cmd := exec.Command(tmpBinary, tt.filename)
			output, err := cmd.CombinedOutput()

			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}

			if tt.errMsg != "" && !strings.Contains(string(output), tt.errMsg) {
				t.Errorf("expected error message containing %q, got: %s", tt.errMsg, output)
			}
		})
	}
}

