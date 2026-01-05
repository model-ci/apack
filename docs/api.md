# apack REST API Documentation

apack provides HTTP-based REST API interfaces supporting model management, task execution, and system monitoring functions. All API endpoints are located under the `/base` namespace.

## Table of Contents

- [Basic Information](#basic-information)
- [Authentication](#authentication)
- [Model Operation APIs](#model-operation-apis)
- [Task Management APIs](#task-management-apis)
- [System Health APIs](#system-health-apis)
- [Error Handling](#error-handling)
- [Request Response Format](#request-response-format)

## Basic Information

### Base URL
```
http://localhost:8080/base
```

### Content Types
- Request: `application/json`
- Response: `application/json`

### Version Control
Current API version: `v1`

## Authentication

API supports Bearer Token authentication:
```http
Authorization: Bearer <token>
```

## Model Operation APIs

### Generate Model Configuration
Generate new model configuration.

```http
POST /base/v1/gen
```

**Request Body:**
```json
{
  "reference": "model:latest",
  "params": {
    "output": "./output",
    "overwrite": true
  }
}
```

**Response:**
```json
{
  "task_id": "gen_123456",
  "code": "success",
  "message": "Configuration generated successfully"
}
```

### Pull Model
Pull model from remote registry.

```http
POST /base/v1/pull
```

**Request Body:**
```json
{
  "reference": "registry.example.com/model:latest",
  "params": {
    "concurrency": 4
  }
}
```

**Response:**
```json
{
  "task_id": "pull_789abc",
  "code": "success",
  "message": "Model pull started"
}
```

### Push Model
Push model to remote registry.

```http
POST /base/v1/push
```

**Request Body:**
```json
{
  "reference": "registry.example.com/model:latest",
  "params": {
    "concurrency": 4
  }
}
```

**Response:**
```json
{
  "task_id": "push_def456",
  "code": "success",
  "message": "Model push started"
}
```

### Build Model
Build model image.

```http
POST /base/v1/build
```

**Request Body:**
```json
{
  "reference": "model:latest",
  "params": {
    "output": "./build",
    "algo": "sha256"
  }
}
```

**Response:**
```json
{
  "task_id": "build_123xyz",
  "code": "success",
  "message": "Build started"
}
```

### Tag Management
Create tags for models.

```http
POST /base/v1/tag
```

**Request Body:**
```json
{
  "reference": "model:latest",
  "params": {
    "target_ref": "model:v1.0"
  }
}
```

**Response:**
```json
{
  "task_id": "tag_789def",
  "code": "success",
  "message": "Tag created successfully"
}
```

### Export Model
Export model to file.

```http
POST /base/v1/export
```

**Request Body:**
```json
{
  "reference": "model:latest",
  "params": {
    "output": "./export/model.tar"
  }
}
```

**Response:**
```json
{
  "task_id": "export_456abc",
  "code": "success",
  "message": "Export started"
}
```

### List Models
List locally available models.

```http
POST /base/v1/list
```

**Request Body:**
```json
{
  "params": {
    "filter": "all"
  }
}
```

**Response:**
```json
{
  "task_id": "list_123",
  "code": "success",
  "message": "[\"model:latest\", \"model:v1.0\"]"
}
```

### Model Information
Get detailed information about a model.

```http
POST /base/v1/info
```

**Request Body:**
```json
{
  "reference": "model:latest"
}
```

**Response:**
```json
{
  "task_id": "info_456",
  "code": "success",
  "message": "{\"size\": \"1.2GB\", \"digest\": \"sha256:abc123...\"}"
}
```

### Inspect Model
Detailed inspection of model contents.

```http
POST /base/v1/inspect
```

**Request Body:**
```json
{
  "reference": "model:latest"
}
```

**Response:**
```json
{
  "task_id": "inspect_789",
  "code": "success",
  "message": "{\"layers\": 5, \"config\": {...}}"
}
```

### Remove Model
Delete local model.

```http
POST /base/v1/remove
```

**Request Body:**
```json
{
  "reference": "model:latest"
}
```

**Response:**
```json
{
  "task_id": "remove_012",
  "code": "success",
  "message": "Model removed"
}
```

## Runtime Operation APIs

### Run Model
Start model instance.

```http
POST /base/v1/run
```

**Request Body:**
```json
{
  "reference": "model:latest",
  "params": {
    "port": "8080",
    "env": {"KEY": "VALUE"}
  }
}
```

**Response:**
```json
{
  "task_id": "run_345",
  "code": "success",
  "message": "Model started on port 8080"
}
```

### View Running Instances
View currently running instances.

```http
POST /base/v1/ps
```

**Request Body:**
```json
{}
```

**Response:**
```json
{
  "task_id": "ps_678",
  "code": "success",
  "message": "[{\"id\": \"instance1\", \"status\": \"running\"}]"
}
```

### Stop Instance
Stop running model instance.

```http
POST /base/v1/kill
```

**Request Body:**
```json
{
  "reference": "instance1"
}
```

**Response:**
```json
{
  "task_id": "kill_901",
  "code": "success",
  "message": "Instance stopped"
}
```

## Task Management APIs

### Get Task Information
Get information about a specific task.

```http
GET /base/v1/tasks/:taskId
```

**Path Parameters:**
- `taskId`: Task ID

**Response:**
```json
{
  "id": "gen_123456",
  "status": "completed",
  "progress": 100,
  "result": "Configuration generated successfully"
}
```

### Stream Task Progress
Real-time task progress streaming.

```http
GET /base/v1/tasks/:taskId/stream
```

**Path Parameters:**
- `taskId`: Task ID

**Response:** (Server-Sent Events)
```
data: {"progress": 50, "message": "Pulling layer 1/3"}

data: {"progress": 100, "message": "Pull completed"}
```

### Cancel Task
Cancel executing task.

```http
POST /base/v1/tasks/:taskId/cancel
```

**Path Parameters:**
- `taskId`: Task ID

**Response:**
```json
{
  "task_id": "gen_123456",
  "code": "success",
  "message": "Task cancelled"
}
```

### List All Tasks
Get list of all tasks.

```http
GET /base/v1/tasks
```

**Response:**
```json
{
  "tasks": [
    {
      "id": "gen_123456",
      "type": "gen",
      "status": "completed",
      "created_at": "2024-01-01T12:00:00Z"
    },
    {
      "id": "pull_789abc",
      "type": "pull",
      "status": "running",
      "created_at": "2024-01-01T12:05:00Z"
    }
  ]
}
```

## System Health APIs

### Health Check
Check system health status.

```http
GET /base/health
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Readiness Check
Check if system is ready.

```http
GET /base/ready
```

**Response:**
```json
{
  "status": "ready",
  "timestamp": "2024-01-01T12:00:00Z"
}
```

## Error Handling

### Error Response Format
```json
{
  "error": {
    "code": "invalid_request",
    "message": "Missing required parameter: reference",
    "details": {
      "parameter": "reference"
    }
  }
}
```

### Common Error Codes
| HTTP Status Code | Error Code | Description |
|------------------|------------|-------------|
| 400 | `invalid_request` | Invalid request parameters |
| 401 | `unauthorized` | Authentication failed |
| 404 | `not_found` | Resource not found |
| 500 | `internal_error` | Server internal error |

## Request Response Format

### Request Format
All POST requests use unified request body format:
```json
{
  "reference": "model reference",
  "reference_str": "model reference string",
  "params": {
    "parameter1": "value1",
    "parameter2": "value2"
  },
  "config_json": "Base64 encoded configuration JSON"
}
```

### Response Format
All operations return task ID and status:
```json
{
  "task_id": "task ID",
  "code": "status code",
  "message": "message content"
}
```

### Task Status
- `pending`: Waiting
- `running`: Executing
- `completed`: Completed
- `failed`: Failed
- `cancelled`: Cancelled

---

*This document is based on apack version: 0.0.1*
*For more information, please refer to: https://github.com/model-ci/apack*