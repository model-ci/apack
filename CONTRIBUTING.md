# Contribution Guidelines

Thank you for your interest in the apack project! We warmly welcome every contribution from the community. Whether you are an experienced developer, AI researcher, operations engineer, or documentation enthusiast, you can contribute to the apack project.

## Table of Contents

- [Ways to Contribute](#ways-to-contribute)
- [Development Guide](#development-guide)
- [Code Standards](#code-standards)
- [Submission Process](#submission-process)
- [Contributor Recognition](#contributor-recognition)
- [Contact Us](#contact-us)
- [Areas Needing Help](#areas-needing-help)

## Ways to Contribute

### Code Contributions
- **Feature Development**: Implement new features, optimize performance, add new inference engine support
- **Bug Fixes**: Fix known issues, improve system stability
- **Test Improvement**: Write unit tests, integration tests, improve code coverage
- **Code Refactoring**: Improve code structure, enhance maintainability

### Documentation Contributions
- **API Documentation**: Improve interface documentation and code comments
- **User Guides**: Write usage tutorials and best practices
- **Development Documentation**: Supplement architecture design and development guides
- **Translation Work**: Assist in translating documentation to other languages

### Community Participation
- **Issue Reporting**: Report bugs or suggest features through GitHub Issues
- **Experience Sharing**: Share usage experiences and best practices in Discussions
- **Model Adaptation**: Help adapt more AI model formats (ONNX, PyTorch, TensorFlow, etc.)
- **Community Building**: Participate in discussions, answer questions, help new users

## Development Guide

### Environment Setup

#### System Requirements
- **Go Version**: 1.21 or higher
- **Operating System**: Linux, macOS, or Windows
- **Git**: For version control
- **Make**: For build scripts

#### Environment Setup
```bash
# 1. Fork the project and clone locally
git clone https://github.com/model-ci/apack.git
cd apack

# 2. Install Go dependencies
go mod download

# 3. Install development tools
make install-tools

# 4. Run tests to ensure environment is working
make test

# 5. Local build
make build

# 6. Verify installation
./bin/apack version
```

### Project Structure

```
apack/
├── cmd/                    # Command-line tool entry points
│   ├── build/             # Build command
│   ├── run/               # Run command
│   └── ...
├── internal/              # Internal packages
│   ├── api/               # API interfaces
│   ├── config/            # Configuration management
│   ├── runtime/           # Runtime
│   └── ...
├── pkg/                   # Public packages
│   ├── infer/             # Inference engine integration
│   └── ...
├── docs/                  # Documentation
├── tests/                 # Test files
└── Makefile               # Build script
```

### Testing

```bash
# Run all tests
make test

# Run unit tests
make test-unit

# Run integration tests
make test-integration

# Generate test coverage report
make test-coverage

# Run benchmark tests
make test-bench
```

## Code Standards

### Go Code Style
- Follow [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` to format code
- Use `golangci-lint` for code checking
- Keep functions concise with single responsibility
- Add appropriate comments and documentation

### Naming Conventions
- **Package Names**: Short, lowercase, singular form
- **Function Names**: CamelCase, visibility indicated by capitalization
- **Variable Names**: CamelCase, concise and clear
- **Constant Names**: UPPERCASE, underscore separated

### Code Checking
```bash
# Format code
make fmt

# Run linter
make lint

# Fix automatically fixable issues
make lint-fix
```

## Submission Process

### Commit Standards
We follow [Conventional Commits](https://www.conventionalcommits.org/) specification:

```bash
# Feature development
git commit -m "feat: add support for ONNX model format"

# Bug fix
git commit -m "fix: resolve memory leak in model loading"

# Documentation update
git commit -m "docs: update quick start guide"

# Performance optimization
git commit -m "perf: optimize model layer caching mechanism"

# Code refactoring
git commit -m "refactor: simplify model loading logic"

# Test related
git commit -m "test: add integration tests for model registry"

# Build related
git commit -m "build: update Go version to 1.21"

# CI/CD related
git commit -m "ci: add automated security scanning"
```

### Pull Request Process

#### 1. Preparation
```bash
# Create feature branch
git checkout -b feature/your-feature-name

# Or create fix branch
git checkout -b fix/issue-number-description
```

#### 2. Development Phase
- Write code following project standards
- Add necessary test cases
- Update relevant documentation
- Ensure all tests pass

#### 3. Pre-Submission Check
```bash
# Run complete check process
make check

# This includes:
# - Code formatting check
# - Lint check
# - Unit tests
# - Integration tests
# - Security scanning
```

#### 4. Create Pull Request
- Describe changes and reasons in detail
- Reference related issue numbers
- Add test result screenshots (if applicable)
- Select appropriate reviewers

#### 5. Code Review
- Maintainers will review your PR promptly
- Make necessary modifications based on feedback
- Maintain positive communication and collaboration

#### 6. Merge and Release
- After review approval, maintainers will merge PR
- Your contribution will be included in the next release

### PR Checklist

Before submitting PR, please ensure:

- [ ] Code follows project coding standards
- [ ] Added necessary test cases
- [ ] All tests pass (`make test`)
- [ ] Code passes lint check (`make lint`)
- [ ] Updated relevant documentation
- [ ] Commit messages follow Conventional Commits specification
- [ ] No new security vulnerabilities introduced
- [ ] No significant performance degradation
- [ ] Backward compatibility ensured

## Contact Us

### Communication Channels

| Channel | Purpose | Link |
|---------|---------|------|
| **GitHub Issues** | Bug reports, feature requests | [Submit Issue](https://github.com/model-ci/apack/issues) |
| **GitHub Discussions** | Technical discussions, experience sharing | [Join Discussion](https://github.com/model-ci/apack/discussions) |

### Getting Help

If you encounter problems during contribution:
1. Check [FAQ](docs/FAQ.md)
2. Search existing Issues and Discussions
3. Ask questions in Discussions
4. Contact project maintainers

## Areas Needing Help

We particularly welcome contributions in the following areas:

### Technical Development
- [ ] **Multi-architecture Support**: ARM64, RISC-V, and other platform adaptations
- [ ] **Inference Engine Expansion**: TensorRT, OpenVINO, MindSpore integration
- [ ] **Cloud Platform Integration**: Kubernetes Operator, Helm Charts development
- [ ] **Performance Optimization**: Model loading speed, memory usage optimization
- [ ] **Security Enhancement**: Image signature verification, access control mechanisms

### Monitoring & Operations
- [ ] **Monitoring Observability**: Prometheus metrics, distributed tracing integration
- [ ] **Logging System**: Structured logging, log aggregation solutions
- [ ] **Automated Testing**: CI/CD pipeline optimization
- [ ] **Deployment Tools**: Docker Compose, K8s deployment templates

### Ecosystem Building
- [ ] **Multi-language SDKs**: Python, JavaScript, Java client libraries
- [ ] **Model Format Support**: More AI framework adaptations
- [ ] **Documentation Improvement**: API documentation, best practice guides
- [ ] **Community Building**: Tutorial creation, case sharing

### User Experience
- [ ] **Web Management Interface**: Model management, monitoring dashboard
- [ ] **CLI Improvements**: Interactive experience, error message optimization
- [ ] **Plugin System**: Extension mechanism design and implementation
- [ ] **Mobile Support**: iOS/Android client development

## Contribution Ideas

If you have the following ideas, we warmly welcome them:

### Innovative Features
- **Automatic Model Optimization**: Smart quantization, pruning suggestions
- **Model Version Management**: Git-like model version control
- **Distributed Building**: Multi-node parallel packaging, continuous distribution
- **Edge Computing**: Mobile devices, IoT device support

### Tool Improvements
- **Performance Benchmarks**: Establish standardized performance test suites
- **Debugging Tools**: Model inference debugging, performance analysis
- **Configuration Management**: Visual configuration editor
- **Batch Processing Tools**: Batch model processing and conversion

### Ecosystem Expansion
- **Cloud Service Integration**: AWS, Azure, GCP native support
- **CI/CD Integration**: Jenkins, GitHub Actions plugins
- **IDE Plugins**: VSCode, IntelliJ extensions
- **Internationalization**: Multi-language interfaces and documentation

## Open Source License

This project uses the [Apache License 2.0](LICENSE). By contributing code, you agree to license your contributions under this license.

## Acknowledgments

Thank you to all developers, users, and supporters who have contributed to the apack project! It is through your participation that apack can continue to develop and improve.

**Let's build a better AI model containerization ecosystem together!**