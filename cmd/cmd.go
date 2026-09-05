package cmd

import (
	"bytes"
	"os/exec"
)

func Exec(c string) (string, error) {
	var cmdout bytes.Buffer
	var cmderr bytes.Buffer

	cmd := exec.Command("bash", "-c", c)
	cmd.Stdout = &cmdout
	cmd.Stderr = &cmderr

	err := cmd.Run()
	if err != nil {
		return cmderr.String(), err
	}

	return cmdout.String(), nil
}
