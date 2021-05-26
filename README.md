# cert-manager-nifcloud-webhook

## Running the test suite

```bash
$ vi testdata/nifcloud-solver/nifcloud-secret.yaml
$ TEST_ZONE_NAME=example.com. make test
```

## Deploy

### cert-manager

See [official document](https://cert-manager.io/docs/installation/kubernetes/).

### nifcloud-webhook

```bash
$ helm repo add cert-manager-nifcloud-webhook https://raw.githubusercontent.com/aokumasan/cert-manager-nifcloud-webhook/master/charts
$ helm install cert-manager-nifcloud-webhook cert-manager-nifcloud-webhook/cert-manager-nifcloud-webhook --namespace cert-manager --version v0.0.1
```

## Create an issuer

The name of solver to use is `nifcloud-solver`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: nifcloud-secret
  namespace: cert-manager
type: Opaque
data:
  access-key-id: <base64 encoded access key id>
  secret-access-key: <base64 encoded secret access key>

---

apiVersion: cert-manager.io/v1alpha2
kind: ClusterIssuer
metadata:
  name: nifcloud-issuer
  namespace: cert-manager
spec:
  acme:
    email: <your email address>
    server: https://acme-staging-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: nifcloud-issuer-account-key
    solvers:
      - dns01:
          webhook:
            groupName: nifcloud.com
            solverName: nifcloud-solver
            config:
              accessKeyIdSecretRef:
                name: nifcloud-secret
                key: access-key-id
              secretAccessKeySecretRef:
                name: nifcloud-secret
                key: secret-access-key
```
