# Frequently Asked Questions (FAQ)

This document collects the most common questions and solutions for apack users. If your question is not in this list, please check [GitHub Issues](https://github.com/model-ci/apack/issues) or ask in [Discussions](https://github.com/model-ci/apack/discussions).

## Table of Contents

- [Installation & Configuration](#installation--configuration)
- [Model Management](#model-management)
- [Runtime Issues](#runtime-issues)
- [Performance Optimization](#performance-optimization)
- [Network & Connectivity](#network--connectivity)
- [Troubleshooting](#troubleshooting)
- [Compatibility Issues](#compatibility-issues)

## Installation & Configuration

### Q: How to install apack?

**A:** apack provides multiple installation methods:

1. **Pre-compiled binaries (recommended)**:
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

2. **Build from source**:
```bash
git clone https://github.com/model-ci/apack.git
cd apack
make build
```

### Q: Which operating systems does apack support?

**A:** apack supports the following operating systems:
- **Linux**: Ubuntu 18.04+, CentOS 7+, Debian 9+
- **macOS**: macOS 10.15+ (Intel and Apple Silicon)
- **Windows**: Windows 10+ (experimental support)

### Q: How to verify apack is working properly after installation?

**A:** Run the following commands to verify installation:
```bash
# Check version
apack version

# Check help information
apack --help

# Test basic functionality
apack ls
```

### Q: How to configure apack's working directory?

**A:** apack uses `~/.apack` as the default working directory. You can customize it in the following ways:

1. **Environment variable**:
```bash
export APACK_ROOT=/path/to/your/workspace
```

2. **Command line parameter**:
```bash
apack --workspace /path/to/your/workspace <command>
```

3. **Configuration file**:
```yaml
# ~/.apack/config.yaml
workspace: /path/to/your/workspace
```

## Model Management

### Q: Which model formats does apack support?

**A:** apack currently supports the following model formats:
- **GGUF**: llama.cpp format, recommended for LLM
- **ONNX**: Cross-platform machine learning format
- **PyTorch**: .pt, .pth formats
- **TensorFlow**: SavedModel, .pb formats
- **Safetensors**: Safe tensor storage format

More format support is under development.

### Q: How to download models from HuggingFace?

**A:** You can use git or HuggingFace CLI:

1. **Using git**:
```bash
git clone https://huggingface.co/microsoft/DialoGPT-medium
cd DialoGPT-medium
apack gen .
```

2. **Using HuggingFace CLI**:
```bash
pip install huggingface_hub
huggingface-cli download microsoft/DialoGPT-medium --local-dir ./DialoGPT-medium
cd DialoGPT-medium
apack gen .
```

### Q: How to create a custom Apackfile?

**A:** Apackfile is a YAML format configuration file, example:

```yaml
ociVersion: 1.0.1
package:
  workspace: ./workspace/Qwen3-0.6B-GGUF
  models:
  - path: Qwen3-0.6B-Q8_0.gguf
  - path: params
  codes:
  - path: .
  docs:
  - path: LICENSE
    description: License file
  - path: README.md
    description: Readme file
spec:
  descriptor:
    authors:
    - qwen
    name: qwen3-0.6b
    description: this is llama
```

### Q: How to manage model versions?

**A:** apack supports semantic version management:

```bash
# Build different versions
apack build -t myregistry/model:v1.0.0 .
apack build -t myregistry/model:v1.1.0 .
apack build -t myregistry/model:latest .

# List all versions
apack ls myregistry/model

# Pull specific version
apack pull myregistry/model:v1.0.0
```

### Q: How to delete unwanted models?

**A:** Use the `remove` command to delete models:

```bash
# Delete specific model
apack remove myregistry/model:v1.0.0

# Delete all versions
apack remove myregistry/model

# Clean up dangling images
apack system prune

# Force delete (skip confirmation)
apack remove --force myregistry/model:v1.0.0
```

## Runtime Issues

### Q: Model startup failed, how to diagnose the problem?

**A:** Follow these steps for diagnosis:

1. **Check logs**:
```bash
apack logs <runtime_id>
```

2. **Check system resources**:
```bash
# Check memory usage
free -h

# Check disk space
df -h
```

3. **Use debug mode**:
```bash
apack run --debug myregistry/model:v1.0.0
```

4. **Check port usage**:
```bash
netstat -tlnp | grep :8080
```

### Q: Model loading is very slow, how to optimize?

**A:** Try the following optimization methods:

1. **Enable memory mapping**:
```bash
apack run --mmap myregistry/model:v1.0.0
```

2. **Use SSD storage**:
```bash
# Move model storage to SSD
apack --data /path/to/ssd
```

3. **Increase thread count**:
```bash
apack run --threads $(nproc) myregistry/model:v1.0.0
```

4. **Use smaller quantization versions**:
```bash
# Use Q4_K_M instead of Q8_0
apack pull myregistry/model:q4_k_m
```

### Q: How to limit resources used by the model?

**A:** Use resource limitation parameters:

```bash
# Limit memory usage
apack run --memory 8g myregistry/model:v1.0.0

# Limit CPU usage
apack run --cpus 4 myregistry/model:v1.0.0

# Limit GPU usage
apack run --gpu 0 --gpu-memory 4g myregistry/model:v1.0.0

# Combine usage
apack run --memory 16g --cpus 8 --gpu 0,1 myregistry/model:v1.0.0
```

### Q: How to debug models?

**A:** Use debugging commands:

```bash
# Run in background
apack run myregistry/model:v1.0.0

# Check running status
apack ps

# View logs
apack logs my-model

# Stop service
apack stop my-model
```

## Performance Optimization

### Q: How to improve inference performance?

**A:** Consider the following optimization strategies:

1. **Hardware optimization**:
   - Use GPU acceleration: `--gpu 0`
   - Increase memory: `--memory 32g`
   - Use SSD storage

2. **Model optimization**:
   - Choose appropriate quantization level (Q4_K_M is usually the best choice)
   - Use smaller context length: `--context-length 2048`

3. **Runtime optimization**:
   - Adjust thread count: `--threads $(nproc)`
   - Enable batch processing: `--batch-size 512`
   - Use memory mapping: `--mmap`

### Q: What to do when GPU memory is insufficient?

**A:** Try the following solutions:

1. **Partial GPU loading**:
```bash
# Load only some layers to GPU
apack run --gpu-layers 20 myregistry/model:v1.0.0
```

2. **Use smaller models**:
```bash
# Use 7B instead of 13B model
apack pull myregistry/model:7b-q4_k_m
```

3. **Adjust GPU memory allocation**:
```bash
# Limit GPU memory usage
apack run --gpu-memory 6g myregistry/model:v1.0.0
```

4. **Use CPU inference**:
```bash
# Use CPU completely
apack run --no-gpu myregistry/model:v1.0.0
```

### Q: How to monitor model performance?

**A:** apack provides multiple monitoring methods:

1. **Real-time monitoring**:
```bash
# View resource usage
apack stats <runtime_id>

# Continuous monitoring
watch -n 1 'apack stats <runtime_id>'
```

2. **Performance metrics**:
```bash
# Get detailed metrics
apack metrics <runtime_id>
```

## Network & Connectivity

### Q: How to configure custom ports?

**A:** Use `--port` parameter or configure in Apackfile:

```bash
# Command line specification
apack run --ports 9090:9091 myregistry/model:v1.0.0
```

### Q: How to enable HTTPS?

**A:** Configure TLS certificates:

```bash
# Use self-signed certificate
apack run --tls --cert server.crt --key server.key myregistry/model:v1.0.0

# Use Let's Encrypt
apack run --tls --auto-cert --domain yourdomain.com myregistry/model:v1.0.0
```

### Q: How to configure reverse proxy?

**A:** Use Nginx or other reverse proxy:

```nginx
# nginx.conf
server {
    listen 80;
    server_name yourdomain.com;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        
        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## Troubleshooting

### Q: How to resolve "permission denied" error?

**A:** This is usually a permission issue:

1. **Check file permissions**:
```bash
# Ensure apack is executable
chmod +x /usr/local/bin/apack

# Check working directory permissions
ls -la ~/.apack/
```

2. **Use sudo for installation**:
```bash
sudo mv apack /usr/local/bin/
```

### Q: How to resolve "model not found" error?

**A:** Check if the model exists:

1. **List local models**:
```bash
apack ls
```

2. **Check model path**:
```bash
apack inspect myregistry/model:v1.0.0
```

3. **Re-pull model**:
```bash
apack pull myregistry/model:v1.0.0
```

### Q: How to resolve connection timeout?

**A:** Try the following solutions:

1. **Increase timeout time**:
```bash
apack run --timeout 300s myregistry/model:v1.0.0
```

2. **Check network connection**:
```bash
# Test network connectivity
curl -I https://hub.docker.com

# Check DNS resolution
nslookup hub.docker.com
```

3. **Configure proxy** (if needed):
```bash
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
```

### Q: How to resolve insufficient memory error?

**A:** Solutions for insufficient memory:

1. **Check system memory**:
```bash
free -h
```

2. **Use smaller models**:
```bash
# Choose smaller quantization version
apack pull myregistry/model:q2_k
```

3. **Increase swap space**:
```bash
# Create swap file
sudo fallocate -l 8G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

4. **Adjust memory limits**:
```bash
# Increase memory limit
apack run --memory 16g myregistry/model:v1.0.0
```

## Compatibility Issues

### Q: What is the relationship between apack and Docker?

**A:** The relationship between apack and Docker:

- **Compatibility**: Images generated by apack are fully OCI-compliant and can be used by Docker
- **Independence**: apack does not depend on Docker, has its own runtime
- **Interoperability**: Can use Docker commands to operate apack images

```bash
# Export as Docker image
apack save myregistry/model:v1.0.0 | docker load

# Import from Docker
docker load myregistry/model:v1.0.0 | apack save
```

### Q: How to use apack in Kubernetes?

**A:** apack supports Kubernetes deployment:

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: infer
  namespace: apack
spec:
  replicas: 2
  selector:
    matchLabels:
      app: llamafile-server
  template:
    metadata:
      labels:
        app: llamafile-server
    spec:
      containers:
      - name: llamafile-server
        image: hub.apack.com/llama/llama-cpp-infer:v1
        ports:
        - containerPort: 62379
          name: llamafile-port
        env:
        - name: MODEL_PATH
          value: "/volume/Qwen3-0.6B-Q8_0.gguf"
        volumeMounts:
        - name: volume
          mountPath: /volume
      imagePullSecrets:
        - name: apack-secret
      volumes:
      - name: volume
        image:
          reference: hub.apack.com/qwen3/qwen3-gguf:v0.6
          pullPolicy: IfNotPresent
```

### Q: How to integrate with CI/CD pipeline?

**A:** apack can be easily integrated into CI/CD pipelines:

```yaml
# .github/workflows/build.yml
name: Build and Deploy Model
on:
  push:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v2
    
    - name: Install apack
      run: |
        curl -L https://github.com/model-ci/apack/releases/latest/download/apack-linux-amd64 -o apack
        chmod +x apack
        sudo mv apack /usr/local/bin/
    
    - name: Build model
      run: |
        apack build -t ${{ secrets.REGISTRY }}/model:${{ github.sha }} .
    
    - name: Push model
      run: |
        echo ${{ secrets.REGISTRY_PASSWORD }} | apack login ${{ secrets.REGISTRY }} -u ${{ secrets.REGISTRY_USERNAME }} --password-stdin
        apack push ${{ secrets.REGISTRY }}/model:${{ github.sha }}
```

### Q: How to migrate existing model deployments? (TBD)

---

## Get More Help

If your question is not in this FAQ, get help through the following methods:

- **GitHub Issues**: [Report bugs or request features](https://github.com/model-ci/apack/issues)
- **GitHub Discussions**: [Community discussions and Q&A](https://github.com/model-ci/apack/discussions)
---

*This FAQ will be continuously updated. If you discover new common questions, welcome to submit PRs or Issues!*