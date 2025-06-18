# KCP Users Controller

A Kubernetes controller built for KCP that creates and manages `User` custom resources and provisions users in an external OpenID Connect (OIDC) Provider.

## Prerequisites
- Go 1.24+
- Kubernetes 1.11+ with KCP
- An external OIDC Provider (e.g., Keycloak, Dex) configured

## Build & Deploy
Build and push the controller image:
```sh
make docker-build docker-push IMG=<registry>/kcp-users-controller:tag
```
Install CRDs and deploy:
```sh
make install
make deploy IMG=<registry>/kcp-users-controller:tag
```

## Usage
Create a `User` resource:
```yaml
apiVersion: kcp.piotrjanik.dev/v1alpha1
kind: User
metadata:
  name: example-user
spec:
  # TODO: set spec fields
```
Apply the sample:
```sh
kubectl apply -f config/samples/kcp_v1alpha1_user.yaml
```
The controller will reconcile the resource and create the user in the external OIDC Provider.

## Cleanup
```sh
kubectl delete -f config/samples/kcp_v1alpha1_user.yaml
make uninstall
```

## Contributing
Contributions welcome! Please open issues and pull requests.

## License
Licensed under the Apache License, Version 2.0.

