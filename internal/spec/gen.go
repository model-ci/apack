package spec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/licensecheck"
	"github.com/model-ci/apack/internal/log"
	"github.com/model-ci/apack/pkg/layerdb"
	modelspec "github.com/modelpack/model-spec/specs-go/v1"
)

type fileType int

const (
	FileTypeWeight fileType = iota
	FileTypeDataset
	FileTypeCode
	FileTypeDocs
	FileTypeConfig
	FileTypeUnknown
)

const (
	OCIVersion = "1.0.1"	
)

var weightsSuffixes = []string{
	".safetensors", ".pkl", ".joblib",
	".bin", ".pth", ".pt", ".mar", ".pt2", ".ptl",
	".pb", ".ckpt", ".tflite", ".tfrecords",
	".npy", ".npz",
	".keras", ".h5", ".caffemodel", ".pmml", ".coreml",
	".gguf", ".ggml", ".ggmf", ".llamafile", ".onnx",
}

var docsSuffixes = []string{
	".md", ".adoc", ".html", ".pdf",
}

var configSuffixes = []string{
	".json", ".yaml", ".xml", ".txt", "params",
}

var datasetSuffixes = []string{
	".tar", ".zip", ".parquet", ".csv",
}

func DefaultArtifactNames() []string {
	return []string{"Apackfile", "apackfile", ".apackfile"}
}

func IsDefaultArtifactName(filename string) bool {
	for _, name := range DefaultArtifactNames() {
		if name == filename {
			return true
		}
	}
	return false
}

func Gen(dir *Directory, a *Artifact) (*Artifact, error) {
	log.Logger.Infof("Generating apackfile in %s", dir.Path)
	artifact := &Artifact{
		OCIVersion: OCIVersion,
		Package:    a.Package,
		Spec:       a.Spec,
	}

	includeCatchallSection := false
	var unprocessedDirPaths []string
	var modelFiles, configFiles []File
	var detectedLicenseType string

	log.Logger.Info("Reading directory contents")
	for _, file := range dir.Files {
		if IsDefaultArtifactName(file.Name) {
			log.Logger.Infof("Skipping apackfile '%s'", file.Name)
			continue
		}

		if strings.HasPrefix(strings.ToLower(file.Name), "readme") {
			log.Logger.Infof("Found readme file '%s'", file.Name)
			artifact.Package.Docs = append(artifact.Package.Docs, Doc{
				Path:        file.Name,
				Description: "Readme file",
			})
			continue
		} else if strings.HasPrefix(strings.ToLower(file.Name), "license") {
			log.Logger.Infof("Found license file '%s'", file.Name)
			artifact.Package.Docs = append(artifact.Package.Docs, Doc{
				Path:        file.Name,
				Description: "License file",
			})
			licenseType, err := detectLicense(file.Path)
			if err != nil {
				log.Logger.Debugf("Error determining license type: %s", err)
				log.Logger.Warn("Unable to determine license type")
			}
			detectedLicenseType = licenseType
			log.Logger.Debugf("Detected license %s for license file", detectedLicenseType)
			continue
		}

		switch determineFileType(file.Path) {
		case FileTypeWeight:
			modelFiles = append(modelFiles, file)
		case FileTypeConfig:
			log.Logger.Infof("Detected config file '%s'", file.Path)
			configFiles = append(configFiles, file)
		case FileTypeDocs:
			artifact.Package.Docs = append(artifact.Package.Docs, Doc{Path: file.Path})
		case FileTypeDataset:
			artifact.Package.DataSets = append(artifact.Package.DataSets, DataSet{Path: file.Path})
		default:
			log.Logger.Infof("File %s is either code or unknown type. Will be added as a catch-all section", file.Path)
			includeCatchallSection = true
		}
	}

	for _, subDir := range dir.Subdirs {
		dirModelFiles, err := addDirToArtifact(artifact, subDir)
		if err != nil {
			log.Logger.Errorf("Failed to determine type for directory %s: %s", subDir.Path, err)
			unprocessedDirPaths = append(unprocessedDirPaths, subDir.Path)
		}
		modelFiles = append(modelFiles, dirModelFiles...)
		continue
	}

	if len(modelFiles) > 0 {
		if err := addModelToArtifact(artifact, modelFiles); err != nil {
			return nil, fmt.Errorf("failed to add model to artifact: %w", err)
		}
		log.Logger.Info("Adding config files as model parts")
		for _, configFile := range configFiles {
			artifact.Package.Models = append(artifact.Package.Models, Model{Path: configFile.Path})
		}
	} else {
		log.Logger.Info("No model detected; adding config files as dataset layers")
		for _, configFile := range configFiles {
			artifact.Package.DataSets = append(artifact.Package.DataSets, DataSet{Path: configFile.Path})
		}
	}

	log.Logger.Infof("Unable to process %d paths in %s", len(unprocessedDirPaths), dir.Path)
	if includeCatchallSection || len(unprocessedDirPaths) > 5 {
		log.Logger.Infof("Adding catch-all code layer to include files in %s", dir.Path)
		artifact.Package.Codes = append(artifact.Package.Codes, Code{Path: "."})
	} else {
		for _, path := range unprocessedDirPaths {
			artifact.Package.Codes = append(artifact.Package.Codes, Code{Path: path})
		}
	}

	if len(artifact.Package.Models) == 1 && detectedLicenseType != "" {
		artifact.Package.Models[0].License = detectedLicenseType
	} else if len(artifact.Package.DataSets) == 1 && detectedLicenseType != "" {
		artifact.Package.DataSets[0].License = detectedLicenseType
	} else if len(artifact.Package.Codes) == 1 && detectedLicenseType != "" {
		artifact.Package.Codes[0].License = detectedLicenseType
	} else if detectedLicenseType != "" {
		log.Logger.Info("Unsure what license applies to, adding to artifact modelspec")
		artifact.Spec.Descriptor.Licenses = append(artifact.Spec.Descriptor.Licenses, detectedLicenseType)
	}

	return artifact, nil
}

func addDirToArtifact(artifact *Artifact, dir Directory) (modelFiles []File, err error) {
	switch dir.Name {
	case "docs":
		log.Logger.Infof("Directory %s interpreted as documentation", dir.Name)
		artifact.Package.Docs = append(artifact.Package.Docs, Doc{
			Path: dir.Path,
		})
		return nil, nil
	case "src", "pkg", "lib", "build":
		log.Logger.Infof("Directory %s interpreted as code", dir.Name)
		artifact.Package.Codes = append(artifact.Package.Codes, Code{Path: dir.Path})
		return nil, nil
	}

	directoryContents := [int(FileTypeUnknown) + 1][]string{}
	for _, subdir := range dir.Subdirs {
		directoryContents[int(FileTypeUnknown)] = append(directoryContents[int(FileTypeUnknown)], subdir.Path)
	}

	var configFiles []File
	for _, file := range dir.Files {
		fileType := determineFileType(file.Name)
		if fileType == FileTypeWeight {
			modelFiles = append(modelFiles, file)
		}
		if fileType == FileTypeConfig {
			configFiles = append(configFiles, file)
		}
		directoryContents[int(fileType)] = append(directoryContents[int(fileType)], file.Path)
	}

	overallFiletype := FileTypeUnknown
	directoryHasMixedContents := false
	for fType, files := range directoryContents {
		if len(files) > 0 && fileType(fType) != FileTypeConfig {
			if overallFiletype != FileTypeUnknown {
				log.Logger.Infof("Detected mixed contents within directory %s", dir.Path)
				directoryHasMixedContents = true
			}
			overallFiletype = fileType(fType)
		}
	}
	if directoryHasMixedContents {
		return modelFiles, fmt.Errorf("mixed content in directory; unable to determine type")
	}
	switch overallFiletype {
	case FileTypeWeight:
		log.Logger.Infof("Interpreting directory %s as a model directory", dir.Path)
		modelFiles = append(modelFiles, configFiles...)
	case FileTypeDataset:
		log.Logger.Infof("Interpreting directory %s as a dataset directory", dir.Path)
		artifact.Package.DataSets = append(artifact.Package.DataSets, DataSet{Path: dir.Path})
	case FileTypeDocs:
		log.Logger.Infof("Interpreting directory %s as a docs directory", dir.Path)
		artifact.Package.Docs = append(artifact.Package.Docs, Doc{Path: dir.Path})
	default:
		log.Logger.Infof("Could not determine type for directory %s", dir.Path)
		return modelFiles, fmt.Errorf("directory should be handled as Code")
	}

	return modelFiles, nil
}

func determineFileType(filename string) fileType {
	if anySuffix(filename, weightsSuffixes) {
		return FileTypeWeight
	}
	if anySuffix(filename, configSuffixes) {
		return FileTypeConfig
	}
	if anySuffix(filename, docsSuffixes) {
		return FileTypeDocs
	}
	if anySuffix(filename, datasetSuffixes) {
		return FileTypeDataset
	}
	return FileTypeUnknown
}

func InferMediaType(filename string, algo layerdb.Algorithm) string {
	ft := determineFileType(filename)
	return mediaType(ft, algo)
}

func DetermineMediaType(ft fileType, algo layerdb.Algorithm) string {
	return mediaType(ft, algo)
}

func OCIDetermineMediaType(algo layerdb.Algorithm) string {
	switch algo {
	case layerdb.Gzip, layerdb.GzipFastest:
		return layerdb.OCILayerGzip
	case layerdb.Zstd:
		return layerdb.OCILayerZstd
	case layerdb.None:
		return layerdb.OCILayer
	}
	return layerdb.OCILayer
}

func CompatibleOCIDetermineMediaType(algo layerdb.Algorithm) string {
	switch algo {
	case layerdb.Gzip, layerdb.GzipFastest:
		return layerdb.ImageLayerGzip
	case layerdb.Zstd:
		return layerdb.ImageLayerZstd
	case layerdb.None:
		return layerdb.ImageLayer
	}
	return layerdb.ImageLayer
}

func mediaType(ft fileType, algo layerdb.Algorithm) string {
	switch ft {
	case FileTypeWeight:
		switch algo {
		case layerdb.Gzip, layerdb.GzipFastest:
			return modelspec.MediaTypeModelWeightGzip
		case layerdb.Zstd:
			return modelspec.MediaTypeModelWeightZstd
		case layerdb.Raw:
			return modelspec.MediaTypeModelWeightRaw
		case layerdb.None:
			return modelspec.MediaTypeModelWeight
		}

	case FileTypeDataset:
		switch algo {
		case layerdb.Gzip, layerdb.GzipFastest:
			return modelspec.MediaTypeModelDatasetGzip
		case layerdb.Zstd:
			return modelspec.MediaTypeModelDatasetZstd
		case layerdb.Raw:
			return modelspec.MediaTypeModelDatasetRaw
		case layerdb.None:
			return modelspec.MediaTypeModelDataset
		}

	case FileTypeCode:
		switch algo {
		case layerdb.Gzip, layerdb.GzipFastest:
			return modelspec.MediaTypeModelCodeGzip
		case layerdb.Zstd:
			return modelspec.MediaTypeModelCodeZstd
		case layerdb.Raw:
			return modelspec.MediaTypeModelCodeRaw
		case layerdb.None:
			return modelspec.MediaTypeModelCode
		}

	case FileTypeConfig:
		switch algo {
		case layerdb.Gzip, layerdb.GzipFastest:
			return modelspec.MediaTypeModelWeightConfigGzip
		case layerdb.Zstd:
			return modelspec.MediaTypeModelWeightConfigZstd
		case layerdb.Raw:
			return modelspec.MediaTypeModelWeightConfigRaw
		case layerdb.None:
			return modelspec.MediaTypeModelWeightConfig
		}

	case FileTypeDocs:
		switch algo {
		case layerdb.Gzip, layerdb.GzipFastest:
			return modelspec.MediaTypeModelDocGzip
		case layerdb.Zstd:
			return modelspec.MediaTypeModelDocZstd
		case layerdb.Raw:
			return modelspec.MediaTypeModelDocRaw
		case layerdb.None:
			return modelspec.MediaTypeModelDoc
		}
	}
	return ""
}

func addModelToArtifact(artifact *Artifact, files []File) error {
	if len(files) == 0 {
		return nil
	}

	for _, file := range files {
		artifact.Package.Models = append(artifact.Package.Models, Model{Path: file.Path})
	}

	return nil
}

func detectLicense(licensePath string) (string, error) {
	license, err := os.ReadFile(licensePath)
	if err != nil {
		return "", fmt.Errorf("failed to read license file: %w", err)
	}
	cov := licensecheck.Scan(license)
	if len(cov.Match) == 0 {
		return "", fmt.Errorf("no license matched license file")
	}
	if len(cov.Match) == 1 {
		return cov.Match[0].ID, nil
	} else {
		return "", fmt.Errorf("multiple licenses matched license file")
	}
}

func anySuffix(query string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(query, suffix) {
			return true
		}
	}
	return false
}

type Directory struct {
	Name    string
	Path    string
	Files   []File
	Subdirs []Directory
}

type File struct {
	Name string
	Path string
	Size int64
}

func DirectoryFromFS(contextDir string) (*Directory, error) {
	return genDirFromPath(".", contextDir)
}

func genDirFromPath(curDir, contextDir string) (*Directory, error) {
	dirName := filepath.Base(curDir)
	result := &Directory{
		Name: dirName,
		Path: curDir,
	}

	fullPath := filepath.Join(contextDir, curDir)
	ds, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", fullPath, err)
	}
	for _, dirEntry := range ds {
		relPath := filepath.Join(curDir, dirEntry.Name())
		fullPath := filepath.Join(contextDir, relPath)
		t := dirEntry.Type()
		switch {
		case t.IsDir():
			dirListing, err := genDirFromPath(relPath, contextDir)
			if err != nil {
				return nil, err
			}
			result.Subdirs = append(result.Subdirs, *dirListing)
		case t.IsRegular():
			info, err := dirEntry.Info()
			if err != nil {
				return nil, fmt.Errorf("failed to stat file %s: %w", fullPath, err)
			}
			result.Files = append(result.Files, File{
				Name: dirEntry.Name(),
				Path: filepath.ToSlash(relPath),
				Size: info.Size(),
			})
		default:
			continue
		}
	}

	return result, nil
}
