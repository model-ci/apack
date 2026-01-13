package distribution

import (
	"context"
	"fmt"
	"os"
)

type Options struct {
	PlainHTTP         bool
	TLSVerify         bool
	CredentialsPath   string
	ClientCertPath    string
	ClientCertKeyPath string
	Concurrency       int
	Proxy             string
	ConfigJSON        []byte
}

func (o *Options) CompleteAndValidate(ctx context.Context, configRoot string, args []string) error {
	o.CredentialsPath = "" /*consts.CredentialsPath(configRoot)*/

	if certPath := os.Getenv(CertEnvVar); certPath != "" {
		o.ClientCertPath = certPath
	}
	if certKeyPath := os.Getenv(CertKeyEnvVar); certKeyPath != "" {
		o.ClientCertKeyPath = certKeyPath
	}
	if o.Concurrency < 1 {
		return fmt.Errorf("invalid argument for concurrency (%d): must be at least 1", o.Concurrency)
	}

	return nil
}

func DefaultOptions(home string) *Options {
	return &Options{
		PlainHTTP:       false,
		TLSVerify:       true,
		CredentialsPath: "",
	}
}
