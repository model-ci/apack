package service

import "github.com/model-ci/apack/pkg/layerdb"

type Config struct {
	Concurrency int
	NoProxy     bool
	Compress    int
}

func (c *Config) GetCompress() layerdb.Algorithm {
	switch c.Compress {
	case 0:
		return layerdb.Gzip
	case 1:
		return layerdb.GzipFastest
	case 2:
		return layerdb.Zstd
	case 3:
		return layerdb.None
	}

	return layerdb.Gzip
}
