package processing

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

var ErrLUTProcessorUnavailable = errors.New("LUT_PROCESSOR_UNAVAILABLE")

// LUTProcessor is owned by the Go agent so tests/recovery can exercise graded
// processing without shelling out. The production adapter uses ImageMagick 7
// (`magick`) because it is a mature Windows-friendly CLI for JPEG/CLUT work;
// .cube support is intentionally treated as operationally variable and any CLI
// failure is persisted as a retryable processing failure.
type LUTProcessor interface {
	Apply(ctx context.Context, inputPath string, lutPath string, outputPath string) error
}

type ImageMagickLUTProcessor struct {
	Command string
	Timeout time.Duration
}

func (p ImageMagickLUTProcessor) Apply(ctx context.Context, inputPath string, lutPath string, outputPath string) error {
	command := strings.TrimSpace(p.Command)
	if command == "" {
		command = "magick"
	}
	if _, err := exec.LookPath(command); err != nil {
		return fmt.Errorf("%w: ImageMagick 7 command %q tidak tersedia", ErrLUTProcessorUnavailable, command)
	}
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// ImageMagick 7 can read many .cube LUTs via cube: and apply them as HALD CLUT.
	// Installs vary by delegate support, so stderr is captured for logs while callers
	// expose only safe operator messages.
	cmd := exec.CommandContext(ctx, command, inputPath, "cube:"+lutPath, "-hald-clut", outputPath)
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("LUT_PROCESSING_TIMEOUT: %w", ctx.Err())
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("LUT_PROCESSING_FAILED: %s", msg)
	}
	return nil
}
