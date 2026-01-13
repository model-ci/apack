package repo

import (
	"github.com/model-ci/apack/pkg/distribution"
	"github.com/model-ci/apack/pkg/layerdb"
	"oras.land/oras-go/v2/registry/remote"
)

func NewRegistry(hostname string, opts *distribution.Options) (*remote.Registry, error) {
	reg, err := remote.NewRegistry(hostname)
	if err != nil {
		return nil, err
	}

	reg.PlainHTTP = opts.PlainHTTP
	reg.ManifestMediaTypes = []string{
		layerdb.ImageManifestMediaType,
		layerdb.OCIImageManifestMediaType,
	}

	credentialStore, err := NewCredentialStore(opts.CredentialsPath)
	if err != nil {
		return nil, err
	}

	authClient, err := ClientWithAuth(credentialStore, opts)
	if err != nil {
		return nil, err
	}
	reg.Client = WrapClient(authClient, false)

	return reg, nil
}

func NewRegistryRemote(hostname string, opts *distribution.Options) (*remote.Registry, error) {
	reg, err := remote.NewRegistry(hostname)
	if err != nil {
		return nil, err
	}

	reg.PlainHTTP = opts.PlainHTTP
	reg.ManifestMediaTypes = []string{
		layerdb.ImageManifestMediaType,
		layerdb.OCIImageManifestMediaType,
	}

	credentialStore, err := NewCredentialStoreFromConfig(opts.ConfigJSON)
	if err != nil {
		return nil, err
	}
	authClient, err := ClientWithAuth(credentialStore, opts)
	if err != nil {
		return nil, err
	}
	reg.Client = WrapClient(authClient, false)

	return reg, nil
}
