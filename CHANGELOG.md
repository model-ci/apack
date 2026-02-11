# Changelog

This document records all significant changes to the apack project.

Format based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project follows [Semantic Versioning](https://semver.org/).

## [0.0.2] - 2026-02-11
### Added
- Support for various cloud imports including Ollama, huggingface
- Supports resume download and concurrent download in segments
- Supports global layer sharing and content addressing
- Remove the llamafile engine and replace it with llama.cpp as the default inference engine
- Multi-model management that supports process-level isolation
- Supports tag and export commands
- Supports Go SDK
- Supports running with specified model parameters

## [0.0.1] - 2025-12-31

### Added
- **Initial Release**: apack cloud-native AI model containerization tool
- **Model Packaging**: Support for packaging AI models as OCI-compatible images
- **Built-in Inference Engine**: Integrated llamafile inference engine, supporting GGUF format
- **CLI Tool**: Complete command-line toolset
  - `apack gen` - Automatically generate Apackfile configuration
  - `apack build` - Build model images
  - `apack run` - Run model instances
  - `apack push/pull` - Push/pull model images
  - `apack ls` - List local models
  - `apack ps` - View running instances
  - `apack kill` - Stop model instances
- **OCI Standard Compliance**: Fully compliant with OCI image specification
- **Layered Storage**: Efficient layered storage system based on LayerDB
- **INFER REST API**: Complete HTTP API interface
  - Model management API
  - Runtime management API
  - Inference API (OpenAI compatible)
  - System monitoring API
- **Multi-platform Support**:
  - Linux (AMD64, ARM64)
  - macOS (Intel, Apple Silicon)
  - Windows (Experimental)

### Supported Features
- **Model Formats**: GGUF (primary support)
- **Inference Engines**: llamafile
- **Image Registries**: Docker Hub, private registries
- **Runtime**: Local runtime, no Docker dependency required
- **Configuration Management**: Declarative Apackfile configuration
- **Network Configuration**: Custom ports, host binding
- **Log Management**: Structured log output

### Technical Features
- **Go Language Development**: High performance, cross-platform
- **Zero Dependency Runtime**: All required components built-in
- **Plugin Architecture**: Support for extending different inference engines
- **Incremental Updates**: Only transfer changed image layers
- **Content Addressing**: Hash-based deduplication storage
- **Concurrency Safety**: Support for multiple model concurrent execution

### Documentation
- **User Guide**: Detailed usage instructions and best practices
- **API Documentation**: Complete REST API reference
- **Architecture Design**: System architecture and design philosophy
- **FAQ**: Frequently Asked Questions
- **Contribution Guidelines**: Developer contribution guide

### Known Limitations
- Currently primarily supports GGUF format models
- Kubernetes requires higher version 1.33+, preferably 1.35
- GPU support requires NVIDIA graphics cards and CUDA
- Windows support is still being improved

---

## Version Information

### Version Number Rules
apack follows [Semantic Versioning](https://semver.org/) specification:

- **Major Version**: Incompatible API changes
- **Minor Version**: Backward-compatible functional additions
- **Patch Version**: Backward-compatible bug fixes

### Release Cycle
- **Major Versions**: Released based on significant feature updates
- **Minor Versions**: Monthly releases, containing new features and improvements
- **Patch Versions**: Released as needed, primarily fixing bugs

### Support Policy
- **Current Version**: Full support, including new features and bug fixes
- **Previous Major Version**: Security updates and critical bug fixes
- **Older Versions**: Security updates only

---

## Contribution Guide

We welcome community contributions! Please see [CONTRIBUTING.md](CONTRIBUTING.md) to learn how to participate in project development.

### How to Report Issues
1. Check [Known Issues](https://github.com/model-ci/apack/issues)
2. Use [Issue Template](https://github.com/model-ci/apack/issues/new/choose) to report new issues
3. Provide detailed reproduction steps and environment information

### How to Request Features
1. Discuss ideas in [Discussions](https://github.com/model-ci/apack/discussions)
2. Create [Feature Request Issue](https://github.com/model-ci/apack/issues/new?template=feature_request.md)
3. Participate in community discussions and voting

---

## Acknowledgments

Thank you to all developers, users, and supporters who have contributed to the apack project!

### Special Thanks
- [llama.cpp](https://github.com/ggerganov/llama.cpp) project for the high-performance inference engine
- [OCI](https://opencontainers.org/) community for container standards
- Go language community for excellent development tools and libraries
- All test users for valuable feedback

---

## Links

- **Project Homepage**: https://github.com/model-ci/apack
- **Issue Reporting**: https://github.com/model-ci/apack/issues
- **Community Discussions**: https://github.com/model-ci/apack/discussions
- **Release Page**: https://github.com/model-ci/apack/releases

---

*This changelog will be continuously updated to record every significant change to the project.*