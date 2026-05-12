// Package pdftotext is a thin wrapper around the `pdftotext` binary from
// poppler-utils. It pipes the input PDF bytes to stdin and returns the
// extracted plaintext from stdout.
package pdftotext

import (
	"bytes"
	"fmt"
	"os/exec"
)

// Extract pipes pdfData through `pdftotext - -` and returns the extracted
// text. Returns an error if the binary is missing, fails to write/read,
// or exits non-zero (stderr of the child is included in the error).
func Extract(pdfData []byte) (string, error) {
	cmd := exec.Command("pdftotext", "-", "-")
	cmd.Stdin = bytes.NewReader(pdfData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := bytes.TrimSpace(stderr.Bytes()); len(msg) > 0 {
			return "", fmt.Errorf("pdftotext: %w: %s", err, msg)
		}
		return "", fmt.Errorf("pdftotext: %w", err)
	}
	return stdout.String(), nil
}
