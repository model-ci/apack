package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"
)

func TimePtr(t time.Time) *time.Time {
	return &t
}

func FileExist(file string) bool {
	_, err := os.Stat(file)
	if err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	}
	panic(err)
}

const (
	ExactArgs = iota
	MinArgs
	MaxArgs
)

func CheckArgs(ctx *cli.Context, expected, checkType int) error {
	var err error
	cmdName := ctx.Command.Name
	switch checkType {
	case ExactArgs:
		if ctx.NArg() != expected {
			err = fmt.Errorf("%s: %q requires exactly %d argument(s)", os.Args[0], cmdName, expected)
		}
	case MinArgs:
		if ctx.NArg() < expected {
			err = fmt.Errorf("%s: %q requires a minimum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	case MaxArgs:
		if ctx.NArg() > expected {
			err = fmt.Errorf("%s: %q requires a maximum of %d argument(s)", os.Args[0], cmdName, expected)
		}
	}
	if err != nil {
		fmt.Printf("Incorrect Usage.\n\n")
		cli.ShowCommandHelp(ctx, cmdName)
		return err
	}
	return nil
}

func PromptForInput(prompt string, isSensitive bool) (string, error) {
	var bytes []byte
	var err error
	if !IsInteractiveSession() {
		return "", fmt.Errorf("attempting to read input from non-terminal")
	}

	fmt.Print(prompt)
	if isSensitive {
		bytes, err = term.ReadPassword(int(syscall.Stdin))
		fmt.Print("\n")
	} else {
		reader := bufio.NewReader(os.Stdin)
		bytes, err = reader.ReadBytes('\n')
	}
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}
	return strings.TrimSpace(string(bytes)), nil
}

func IsInteractiveSession() bool {
	return term.IsTerminal(int(syscall.Stdin))
}
