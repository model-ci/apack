package utils

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"
	"oras.land/oras-go/v2/registry"
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

// buildRepositoryManifestURL builds the URL for accessing the manifest API.
// Format: <scheme>://<registry>/v2/<repository>/manifests/<digest_or_tag>
// Reference: https://distribution.github.io/distribution/spec/api/#manifest
func BuildRepositoryManifestURL(plainHTTP bool, ref registry.Reference) string {
	return strings.Join([]string{
		buildRepositoryBaseURL(plainHTTP, ref),
		"manifests",
		ref.Reference,
	}, "/")
}

// buildRepositoryBlobURL builds the URL for accessing the blob API.
// Format: <scheme>://<registry>/v2/<repository>/blobs/<digest>
// Reference: https://distribution.github.io/distribution/spec/api/#blob
func BuildRepositoryBlobURL(plainHTTP bool, ref registry.Reference) string {
	return strings.Join([]string{
		buildRepositoryBaseURL(plainHTTP, ref),
		"blobs",
		ref.Reference,
	}, "/")
}

// buildRepositoryBaseURL builds the base endpoint of the remote repository.
// Format: <scheme>://<registry>/v2/<repository>
func buildRepositoryBaseURL(plainHTTP bool, ref registry.Reference) string {
	return fmt.Sprintf("%s://%s/v2/%s", buildScheme(plainHTTP), ref.Host(), ref.Repository)
}

// buildScheme returns HTTP scheme used to access the remote registry.
func buildScheme(plainHTTP bool) string {
	if plainHTTP {
		return "http"
	}
	return "https"
}

func GetFinalDownloadURL(ctx context.Context, initialURL string) (string, error) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, initialURL, nil)
	if err != nil {
		return "", err
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusFound {
		location := resp.Header.Get("Location")
		if location != "" {
			return location, nil
		}
	}

	return initialURL, nil
}
