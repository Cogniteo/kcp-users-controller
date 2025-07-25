# KCP Users Controller

A Kubernetes controller for managing KCP users through AWS Cognito integration. This controller creates and manages `User` custom resources and automatically provisions users in AWS Cognito User Pools.

## Features

- 🚀 **Automated User Management**: Create, update, and delete users in AWS Cognito via Kubernetes CRDs
- 🔐 **AWS Cognito Integration**: Seamless integration with AWS Cognito User Pools
- 📦 **Multi-platform Docker Images**: Support for AMD64 and ARM64 architectures
- 🤖 **Automated Releases**: CI/CD pipeline with automatic versioning and Docker image publishing
- 🔧 **Kubernetes Native**: Built using controller-runtime framework

## Prerequisites

- Go 1.24+
- Kubernetes 1.11+
- AWS Cognito User Pool configured
- AWS credentials configured (IAM role, access keys, or instance profile)

## Installation

### Using Docker Image

The controller is available as a multi-platform Docker image:

```bash
docker pull ghcr.io/cogniteo/kcp-users-controller:latest
```

### From Source

1. Clone the repository:
```bash
git clone https://github.com/Cogniteo/kcp-users-controller.git
cd kcp-users-controller
```

2. Build and install CRDs:
```bash
make install
```

3. Deploy the controller:
```bash
make deploy IMG=ghcr.io/cogniteo/kcp-users-controller:latest
```

## Configuration

Configure AWS credentials and Cognito settings through environment variables, command-line flags, or Kubernetes secrets:

### Environment Variables

```yaml
env:
- name: AWS_REGION
  value: "us-west-2"
# Use either COGNITO_USER_POOL_ID or COGNITO_USER_POOL_NAME (not both)
- name: COGNITO_USER_POOL_ID
  value: "us-west-2_example123"
# OR
- name: COGNITO_USER_POOL_NAME
  value: "my-user-pool"
- name: METRICS_BIND_ADDRESS
  value: ":8443"
- name: HEALTH_PROBE_BIND_ADDRESS
  value: ":8081"
- name: LEADER_ELECT
  value: "true"
# Add other configuration as needed
```

### Command-Line Flags

All configuration options can also be set via command-line flags:

```bash
# Using User Pool ID
./kcp-users-controller --cognito-user-pool-id=us-west-2_example123 --metrics-bind-address=:8443

# OR using User Pool Name
./kcp-users-controller --cognito-user-pool-name=my-user-pool --metrics-bind-address=:8443
```

Use `--help` to see all available flags and their corresponding environment variables.

## Usage

### Creating a User

Create a `User` custom resource:

```yaml
apiVersion: kcp.cogniteo.io/v1alpha1
kind: User
metadata:
  name: john-doe
  namespace: default
spec:
  email: "john.doe@example.com"
  enabled: true
```

Apply the resource:
```bash
kubectl apply -f user-example.yaml
```

The controller will automatically:
1. Create the user in AWS Cognito User Pool with the specified email
2. Set the user's enabled status
3. Update the User resource status with the user's `sub` (unique identifier)

### Viewing Users

List all users:
```bash
kubectl get users
```

Get detailed information:
```bash
kubectl describe user john-doe
```

### Deleting Users

Delete a user (this will also remove it from Cognito):
```bash
kubectl delete user john-doe
```

## Development

### Local Development

1. Install dependencies:
```bash
go mod tidy
```

2. Run tests:
```bash
make test
```

3. Run the controller locally:
```bash
make run
```

### Building

Build the binary:
```bash
make build
```

Build Docker image:
```bash
make docker-build IMG=your-registry/kcp-users-controller:tag
```

## API Reference

### User Spec

| Field | Type | Description |
|-------|------|-------------|
| `email` | string | User's email address |
| `enabled` | bool | Whether the user is enabled (optional, defaults to false) |

### User Status

| Field | Type | Description |
|-------|------|-------------|
| `sub` | string | User's unique identifier (subject) in the user pool |
| `userPoolStatus` | string | Current status of the user in the user pool |
| `lastSyncTime` | *metav1.Time | Timestamp of the last successful sync with the user pool |
| `conditions` | []metav1.Condition | Current service state conditions of the User |

## Releases

This project uses automated semantic versioning. Releases are automatically created when:
- `feat:` commits trigger minor version bumps
- `fix:` commits trigger patch version bumps
- `BREAKING CHANGE:` commits trigger major version bumps

Docker images are automatically built and pushed to `ghcr.io/cogniteo/kcp-users-controller` with each release.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass: `make test`
6. Submit a pull request

### Commit Message Format

Follow conventional commits:
- `feat: add new feature`
- `fix: resolve bug`
- `docs: update documentation`
- `chore: maintenance tasks`

## License

Licensed under the Apache License, Version 2.0. See [LICENSE](LICENSE) for details.

## Support

For issues and questions:
- 🐛 [Report bugs](https://github.com/Cogniteo/kcp-users-controller/issues)
- 💡 [Request features](https://github.com/Cogniteo/kcp-users-controller/issues)
- 📖 [Documentation](https://github.com/Cogniteo/kcp-users-controller/wiki)
