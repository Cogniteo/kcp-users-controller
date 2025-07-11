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

Configure AWS credentials and Cognito settings through environment variables or Kubernetes secrets:

```yaml
env:
- name: AWS_REGION
  value: "us-west-2"
- name: COGNITO_USER_POOL_ID
  value: "us-west-2_example"
# Add other AWS configuration as needed
```

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
  email: john.doe@example.com
  username: johndoe
  temporaryPassword: TempPass123!
  # Additional user attributes
```

Apply the resource:
```bash
kubectl apply -f user-example.yaml
```

The controller will automatically:
1. Create the user in AWS Cognito User Pool
2. Set the temporary password
3. Update the User resource status

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
| `username` | string | Username for the user |
| `temporaryPassword` | string | Temporary password (optional) |
| `attributes` | map[string]string | Additional user attributes |

### User Status

| Field | Type | Description |
|-------|------|-------------|
| `cognitoStatus` | string | Status in Cognito (CONFIRMED, UNCONFIRMED, etc.) |
| `conditions` | []Condition | Current conditions of the user |

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