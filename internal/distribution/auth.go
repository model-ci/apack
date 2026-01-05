package distribution

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"github.com/model-ci/apack/internal/consts"

	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
	"oras.land/oras-go/v2/registry/remote/retry"
)

func NewCredentialStore(storePath string) (credentials.Store, error) {
	existingCredStore, err := credentials.NewStore(storePath, credentials.StoreOptions{
		DetectDefaultNativeStore: true,
		AllowPlaintextPut:        true,
	})
	if err != nil {
		return nil, err
	}

	storeOpts := credentials.StoreOptions{}
	dockerCredStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return nil, err
	}

	return credentials.NewStoreWithFallbacks(existingCredStore, dockerCredStore), nil
}

func NewCredentialStoreFromConfig(cfg []byte) (credentials.Store, error) {
	existingCredStore, err := credentials.NewMemoryStoreFromDockerConfig(cfg)
	if err != nil {
		return nil, err
	}

	storeOpts := credentials.StoreOptions{}
	dockerCredStore, err := credentials.NewStoreFromDocker(storeOpts)
	if err != nil {
		return nil, err
	}

	return credentials.NewStoreWithFallbacks(existingCredStore, dockerCredStore), nil
}

func ClientWithAuth(store credentials.Store, opts *Options) (*auth.Client, error) {
	client, err := DefaultClient(opts)
	if err != nil {
		return nil, err
	}
	client.Credential = credentials.Credential(store)

	return client, nil
}

func DefaultClient(opts *Options) (*auth.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = !opts.TLSVerify
	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)
		if err != nil {
			return nil, fmt.Errorf("invalid proxy URL: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	if opts.ClientCertKeyPath != "" && opts.ClientCertPath != "" {
		cert, err := tls.LoadX509KeyPair(opts.ClientCertPath, opts.ClientCertKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read certificate: %w", err)
		}
		transport.TLSClientConfig.Certificates = append(transport.TLSClientConfig.Certificates, cert)
	}

	client := &auth.Client{
		Client: &http.Client{
			Transport: retry.NewTransport(transport),
		},
		Cache: auth.NewCache(),
		Header: http.Header{
			"User-Agent": {"apack/" + consts.GetVersion()},
		},
	}

	return client, nil
}

type Cred struct {
	Username string
	Password string
}

func CredentialsLogin(ctx context.Context, store credentials.Store, reg *remote.Registry, cred Cred) error {
	return credentials.Login(ctx, store, reg, auth.Credential{
		Username: cred.Username,
		Password: cred.Password,
	})
}

func CredentialsLogout(ctx context.Context, store credentials.Store, registry string) error {
	return credentials.Logout(ctx, store, registry)
}
