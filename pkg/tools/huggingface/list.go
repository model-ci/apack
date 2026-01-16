package huggingface

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/internal/spec"
)

type hfTreeResponse []struct {
	Type string `json:"type"`
	OID  string `json:"oid"`
	Size int64  `json:"size"`
	Path string `json:"path"`
}

type hfErrorResponse struct {
	Error string `json:"error"`
}

func walkRepoTree(ctx context.Context, client *http.Client, token string, repoBaseUrl *url.URL, subDir string) (*spec.Directory, error) {
	curUrl := *repoBaseUrl
	curUrl.Path = path.Join(curUrl.Path, subDir)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, curUrl.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error calling API: %w", err)
	}

	repoTree, err := processTreeResponse(resp)
	if closeErr := resp.Body.Close(); closeErr != nil {
		log.Logger.Warnf("failed to close response body: %s", closeErr)
	}
	if err != nil {
		return nil, err
	}

	dir := &spec.Directory{
		Name: path.Base(subDir),
		Path: subDir,
	}
	for _, elem := range *repoTree {
		switch elem.Type {
		case "directory":
			subDir, err := walkRepoTree(ctx, client, token, repoBaseUrl, elem.Path)
			if err != nil {
				return nil, err
			}
			dir.Subdirs = append(dir.Subdirs, *subDir)
		case "file":
			name := path.Base(elem.Path)
			if name == ".gitignore" || name == ".gitattributes" {
				continue
			}
			dir.Files = append(dir.Files, spec.File{
				Name: name,
				ID:   elem.OID,
				Path: elem.Path,
				Size: elem.Size,
			})
		default:
			return nil, fmt.Errorf("unknown type in repository tree: %s", elem.Type)
		}
	}

	return dir, nil
}

func processTreeResponse(resp *http.Response) (*hfTreeResponse, error) {
	if resp.StatusCode != http.StatusOK {
		errResp := &hfErrorResponse{}
		if err := json.NewDecoder(resp.Body).Decode(errResp); err != nil {
			return nil, fmt.Errorf("failed to parse API error response: %w", err)
		}
		if resp.StatusCode == http.StatusNotFound && strings.HasPrefix(errResp.Error, "Invalid rev id") {
			ref := errResp.Error[strings.LastIndex(errResp.Error, " ")+1:]
			return nil, fmt.Errorf("reference '%s' not found", ref)
		}
		return nil, fmt.Errorf("got error code %d from API: %s", resp.StatusCode, errResp.Error)
	}

	repoTree := &hfTreeResponse{}
	if err := json.NewDecoder(resp.Body).Decode(repoTree); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}
	return repoTree, nil
}
