package goclip

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

func imageCommand(ctx context.Context, name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	data, readErr := io.ReadAll(io.LimitReader(stdout, maxImageBytes+1))
	if readErr != nil || len(data) > maxImageBytes {
		cancel()
	}
	waitErr := cmd.Wait()
	if readErr != nil {
		return nil, readErr
	}
	if len(data) > maxImageBytes {
		return nil, fmt.Errorf("clipboard image exceeds size limit")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if waitErr != nil {
		return nil, fmt.Errorf("reading clipboard via %s: %w", name, waitErr)
	}
	return data, nil
}
