# apack User Guide

Welcome to apack! This guide will help you deeply understand all the features of apack, from basic operations to advanced usage, enabling you to fully utilize this powerful AI model containerization tool.

## Table of Contents

- [Basic Concepts](#basic-concepts)
- [Installation & Configuration](#installation--configuration)
- [Basic Operations](#basic-operations)
- [Troubleshooting](#troubleshooting)
- [Getting Help](#getting-help)

## Basic Concepts

### What is apack?

apack is an AI model packaging and distribution tool for the cloud-native era. It packages AI models into OCI-standard container images, allowing models to be managed, distributed, and deployed like Docker images.

### Core Concepts

#### Apackfile
Apackfile is apack's build configuration file, similar to Dockerfile. It defines how to package model files into images.

#### Model Image
A model image is an OCI-standard image containing AI models, runtime environments, and dependencies.

#### Inference Engine
apack includes built-in inference engines (such as llama.cpp) that can run models directly without additional configuration.

## Installation & Configuration

### System Requirements

#### Local Environment

| Component | Minimum Requirements | Recommended Configuration |
|-----------|----------------------|---------------------------|
| Operating System | Linux/macOS/Windows | Linux Ubuntu 20.04+ |
| Memory | 4GB RAM | 16GB+ RAM |
| Storage | 10GB available space | 50GB+ SSD |
| Network | Stable internet connection | Bandwidth ≥ 10Mbps |

#### Cloud Environment

| Component | Minimum Requirements | Recommended Configuration |
|-----------|----------------------|---------------------------|
| Container Runtime | Docker 20.10+ or Containerd 1.6+ | Docker 24.0+ |
| Kubernetes | v1.33 | v1.35 |
| Operating System | Kernel 4.5 | Kernel 5.1+ |
| Memory | 128GB RAM | 512GB+ RAM |
| Storage | 500GB available space | 5TB+ SSD |
| Network | Enterprise-grade network connection | Bandwidth ≥ 100Mbps |
| Image Registry | OCI-compliant registry | Harbor/AWS ECR/Azure ACR |

### Installation Steps

#### 1. Download Installation Package
```bash
# Linux AMD64
curl -L https://github.com/model-ci/apack/releases/latest/download/apack-linux-amd64 -o apack
chmod +x apack
sudo mv apack /usr/local/bin/

# macOS Apple Silicon
curl -L https://github.com/model-ci/apack/releases/latest/download/apack-darwin-arm64 -o apack
chmod +x apack
sudo mv apack /usr/local/bin/
```

#### 2. Verify Installation
```bash
apack version
```

#### 3. Configure Image Registry
```bash
# Login to Docker Hub
apack login hub.docker.com

# Or login to private registry
apack login your-registry.com
```

## Basic Operations

#### 1. Login to Image Registry
```bash
# Login to Docker Hub (or your private registry)
apack login hub.docker.com
# Enter username and password
```

#### 2. Get Model Files
```bash
# Clone HuggingFace model (using llama3-1b-GGUF as example)
git clone https://huggingface.co/llama/llama3-1b-GGUF
cd llama3-1b-GGUF
```

#### 3. Generate Apackfile
```bash
# Automatically analyze model directory and generate build configuration file
apack gen ./llama3-1b-GGUF
# Apackfile generated in model directory
```

#### 4. Build Model Image
```bash
# Package model into OCI image
apack build -t hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M ./llama3-1b-GGUF/Apackfile
# Building image...
# Build completed!
```

#### 5. View Local Images
```bash
apack ls
```
```
REPOSITORY               TAG     MAINTAINER   NAME     SIZE       DIGEST
myllama3/llama3-1b-gguf  Q4_K_M  [meta]       llama3   880 MiB    052fa2f75f2a95582a8fb2fa9563ec2c8c043d94cea5b8354c646ca7ca393bc5
```

#### 6. Run Model Locally
```bash
# Start model inference service (built-in llama.cpp engine)
apack run hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M
# Model service started, access http://localhost:51015
```

#### 7. Check Running Status
```bash
apack ps
```
```
RUNTIME ID     MODEL                                        CREATE                          STATUS         ENDPOINTS        NAMES
4c13c2f1a816   hub.docker.com/myllama3/llama3-gguf:Q4_K_M   2025-12-18 18:23:50 +0800 CST   Up 6 seconds   :51015, :51014   llama3-service
```

#### 8. Push to Registry (Optional)
```bash
# Push model image to remote registry
apack push hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M
# Push completed, others can retrieve via apack pull
```

### More Examples

#### Quick Experience with Different Quantization Versions:
```bash
# Pull and run pre-built model image
apack pull hub.docker.com/apack/llama2-7b:Q4_K_M
apack run hub.docker.com/apack/llama2-7b:Q4_K_M --port 8080
```

#### Batch Model Management:
```bash
# List all local model images
apack ls

# Delete unwanted models
apack remove hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M

# Stop running model service
apack kill 4c13c2f1a816
```

#### Advanced Configuration Examples:
```bash
# Run with specified GPU and memory limits
apack run hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M \
  --gpu 0 \
  --memory 8g \
  --port 8080 \
  --name my-llama-service

# Use custom configuration file
apack run hub.docker.com/myllama3/llama3-1b-gguf:Q4_K_M \
  --config ./model-config.yaml
```

### Troubleshooting

**Common Issues and Solutions:**

- **Model Download Failure**: Check network connection, or use HuggingFace mirror sites
- **Insufficient Memory**: Choose smaller quantization versions (e.g., Q2_K instead of Q4_K_M)
- **Port Conflict**: Use `--ports` parameter to specify other ports
- **Permission Issues**: Ensure sufficient disk space and file read/write permissions

### Getting Help

If you encounter problems, you can get help through the following methods:

1. **Submit Issue**: [https://github.com/model-ci/apack/issues](https://github.com/model-ci/apack/issues)
2. **Community Discussion**: [https://github.com/model-ci/apack/discussions](https://github.com/model-ci/apack/discussions)
3. **Check FAQ**: [docs/FAQ.md](FAQ.md)

---

*This guide will be continuously updated. If you have suggestions or issues, please provide feedback!*