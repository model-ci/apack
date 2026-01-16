package spec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/moby/patternmatcher"
)

type Ignore interface {
	Matches(path, layer string) (bool, error)
	HasExclusions() bool
}

type ignore struct {
	ignoreFileMatcher *patternmatcher.PatternMatcher
	layers            []string
}

func (i *ignore) Matches(path, layerPath string) (bool, error) {
	path = cleanPath(path)
	layerPath = cleanPath(layerPath)
	ignoreFileMatches, err := i.ignoreFileMatcher.MatchesOrParentMatches(path)
	if err != nil {
		return false, err
	}
	if ignoreFileMatches {
		return true, nil
	}

	for _, layer := range i.layers {
		layer = cleanPath(layer)
		if strings.HasPrefix(layerPath, layer) {
			continue
		}
		if strings.HasPrefix(path, layer) {
			return true, nil
		}
	}

	return false, nil
}

func (i *ignore) HasExclusions() bool {
	return i.ignoreFileMatcher.Exclusions()
}

func cleanPath(path string) string {
	return filepath.Clean(strings.TrimSpace(path))
}

func NewIgnore(paths []string, artifact *Artifact, extraLayers ...string) (Ignore, error) {
	paths = append(paths, DefaultArtifactNames()...)
	paths = append(paths, IgnoreFileName)
	pm, err := patternmatcher.New(paths)
	if err != nil {
		return nil, fmt.Errorf("invalid %s file: %w", IgnoreFileName, err)
	}

	layerPaths := LayerPaths(artifact)
	layerPaths = append(layerPaths, extraLayers...)
	return &ignore{
		ignoreFileMatcher: pm,
		layers:            layerPaths,
	}, nil
}

func LayerPaths(artifact *Artifact) []string {
	cleanPath := func(path string) string {
		return filepath.Clean(strings.TrimSpace(path))
	}

	var layerPaths []string
	for _, code := range artifact.Package.Codes {
		layerPaths = append(layerPaths, cleanPath(code.Path))
	}

	for _, dataset := range artifact.Package.DataSets {
		layerPaths = append(layerPaths, cleanPath(dataset.Path))
	}

	for _, docs := range artifact.Package.Docs {
		layerPaths = append(layerPaths, cleanPath(docs.Path))
	}

	for _, model := range artifact.Package.Models {
		layerPaths = append(layerPaths, cleanPath(model.Path))
	}

	return layerPaths
}
