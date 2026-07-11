package main

import (
	"daswetter/cmd"
	"os"
	"path/filepath"
	_ "time/tzdata"
)

func main() {
	if interactiveAlias() {
		cmd.ExecuteInteractive()
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "wetter" {
		os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	}
	cmd.Execute()
}

func interactiveAlias() bool {
	if len(os.Args) == 2 && os.Args[1] == "wetter" && terminal(os.Stdin) && terminal(os.Stdout) {
		return true
	}
	name := filepath.Base(os.Args[0])
	return len(os.Args) == 1 && name == "das-wetter" && terminal(os.Stdin) && terminal(os.Stdout)
}

func terminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
