package client

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/apptainer/apptainer/internal/pkg/buildcfg"
	"github.com/google/uuid"
)

func ConvertSifToSandbox(directTo, src, pullTo string) error {
	if directTo != "" {
		// rename the pulled sif first and extract to the sandbox dir
		name := filepath.Base(src)
		newPath := filepath.Join(filepath.Dir(src), name+"-"+uuid.NewString())
		if err := os.Rename(src, newPath); err != nil {
			return fmt.Errorf("unable to rename pulled sif: %v", err)
		}
		defer os.Remove(newPath)
		src = newPath
	}
	// using pulled sif
	exe := filepath.Join(buildcfg.BINDIR, "apptainer")
	cmdArgs := []string{"build", "-F", "--sandbox", pullTo, src}
	cmd := exec.Command(exe, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("while converting cached sif to sandbox: %v", err)
	}
	return nil
}
