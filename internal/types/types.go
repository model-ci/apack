//go:generate easyjson -all types.go
package types

import (
	"encoding/json"
	"io"

	"github.com/model-ci/apack/internal/spec"
	"github.com/model-ci/apack/pkg/layerdb"
	"oras.land/oras-go/v2/registry"
)

const (
	System = "apack"
	Daemon = "daemon"
	Base   = "base"
)

const (
	Byte = 1
	KB   = 1 << (10 * iota) // 1 << (10 * 1) = 1024
	MB                      // 1 << (10 * 2) = 1,048,576
	GB                      // 1 << (10 * 3) = 1,073,741,824
	TB                      // 1 << (10 * 4) = 1,099,511,627,776
	PB                      // 1 << (10 * 5) = 1,125,899,906,842,624
)

type HealthStatus struct {
	Status string `json:"status"`
}

type Login struct {
	Logout
	Username string
	Password string
	Auth     Authentication
}

func (li *Login) Decode(rr io.Reader) error {
	return decoder(rr, li)
}

type Logout struct {
	Home     string
	Registry string
}

func (lo *Logout) Decode(rr io.Reader) error {
	return decoder(rr, lo)
}

type Params struct {
	Overwrite bool
	Algo      layerdb.Algorithm
	Output    string
	TargetRef registry.Reference
}

type Import struct {
	User     string
	Password string
	Tool     string
	Endpoint string
	Repo     string
	Branch   string
}

type Request struct {
	Params
	Import

	ID           string
	ReferenceStr string
	Reference    registry.Reference
	Artifact     spec.Artifact
	ConfigJSON   []byte
}

func (a *Request) Decode(rr io.Reader) error {
	return decoder(rr, a)
}

const (
	StatusSuccess = "success"
	StatusFailure = "failure"
)

type Response struct {
	TaskID  string `json:"task_id"`
	Code    string `json:"code"`
	Message []byte `json:"message"`
}

func (r *Response) Decode(rr io.Reader) error {
	return decoder(rr, r)
}

func decoder(rr io.Reader, j json.Unmarshaler) error {
	b, err := io.ReadAll(rr)
	if err != nil {
		return err
	}
	return j.UnmarshalJSON(b)
}

type Description struct {
	Action    string
	Reference string
	Digest    string
	Size      string
}

type Artifacts struct {
	Count int
	Items []spec.Artifact
}

type Inspect struct {
	Manifest layerdb.Manifest
	Config   layerdb.Config
	Artifact spec.Artifact
}

type Authentication struct {
	PlainHTTP         bool
	TLSVerify         bool
	CredentialsPath   string
	ClientCertPath    string
	ClientCertKeyPath string
	Concurrency       int
	Proxy             string
}
