# Spyre DRA Driver

**Spyre DRA Driver** implements a Kubernetes Dynamic Resource Allocation (DRA) driver that discovers, manages, prepares, and allocates IBM AIU Spyre cards to user Pods.

- [Support Platforms](#support-platform)
- [Demo on Kind cluster](#demo-on-kind-cluster)
  - [Create cluster and deploy driver](#create-cluster-and-deploy-driver)
  - [Deploy examples](#deploy-examples)
  - [Clean up](#clean-up)
- [Development Guide](#development-guide)
  - [Image build](#image-build)
  - [Pre-commit hook](#pre-commit-hook)

## Support Platforms

* **Kubernetes 1.33** with DynamicResourceAllocation [Feature Gates](https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/) and CDI enabled.

  Check [example kind configuration](./hack/kind/kind-cluster-config.yaml).

* **OpenShift 4.20.1**

  ```yaml
  apiVersion: config.openshift.io/v1
  kind: FeatureGate
  metadata:
    name: cluster
  spec:
    featureSet: TechPreviewNoUpgrade
  ```

## Demo on Kind cluster

### Create cluster and deploy driver

1. Build image

    ```bash
    make docker-build
    ```

2. Prepare cluster

    - For testing on Kind cluster, create a `kind` cluster to run by

      ```bash
      make create-cluster
      ```

      To load driver image to Kind cluster, run

      ```bash
      make load-driver
      ```

    - For cluster with more than one node, you need to set nodeSelector.kubernetes.io/hostname to see the same result (one running and one pending pod). Otherwise, the pod can will be scheduled and running on the different node.

3. Install the example resource driver

    Run:

    ```bash
    make deploy
    ```

    This target will deploy a driver via helm.

    After deployment, you should see the `ResourceSlice`.

    ```bash
    kubectl get resourceslices -oyaml
    ```

    [Example generated ResourceSlice](demo/generated-resourceslices.yaml)

### Deploy examples

use case|available|example
---|---|---
Requesting a single Spyre card|:heavy_check_mark:|[Example 1](#example-1-general-request)
Requesting a specific Spyre card|:heavy_check_mark:|[Example 2](#example-2-specify-a-specific-pci-address)
Requesting 2 Spyre card within the same NUMA|:heavy_check_mark:|[Example 3](#example-3-specify-a-numa-constraint)

> [!TIP] Must clean pods from each example before start testing the new example.

#### Example 1: general request

  ```bash
  # for kind
  kubectl apply -f ./demo/kind/spyre-test1.yaml
  # for OpenShift
  kubectl apply -f ./demo/ocp/spyre-test1.yaml
  ```

  Check resource claim:

  ```bash
  kubectl get resourceclaim -n spyre-test1
  ```

  Investigate senlib_config inside pod:

  ```bash
  # kubectl exec -it pod0 -n spyre-test1 -- bash
  > root@pod0:/# ls /etc/aiu/
  senlib_config.json  topo.json
  > root@pod0:/# cat /etc/aiu/senlib_config.json
  {"GENERAL":{"sen_bus_id":["0000:3d:00.0"],"target":"SOC"} ...

  # kubectl exec -it pod1 -n spyre-test1 -- bash
  > root@pod1:/# cat /etc/aiu/senlib_config.json
  {"GENERAL":{"sen_bus_id":["0000:1e:00.0"],"target":"SOC"} ...
  ```

#### Example 2: specify a specific PCI address

  ```yaml
  apiVersion: resource.k8s.io/v1beta1
  kind: ResourceClaimTemplate
  ...
  spec:
    spec:
      devices:
        requests:
        - name: spyre
          deviceClassName: spyre.ibm.com
          selectors:
          - cel:
            expression: |-
              device.attributes["spyre.ibm.com"].pciAddress == "0000:1a:00.0"
  ```

  [Example](./demo/kind/spyre-test2.yaml)

  ```bash
  # for kind
  kubectl apply -f ./demo/kind/spyre-test2.yaml
  # for OpenShift
  kubectl apply -f ./demo/ocp/spyre-test2.yaml
  ```

  Check pods (Only one pod can be allocated. the other pod should be pending.)

  ```bash
  # kubectl get po -n spyre-test2
  NAME   READY   STATUS    RESTARTS   AGE
  pod0   1/1     Running   0          2m5s
  pod1   0/1     Pending   0          6s
  ```

  > For cluster with more than one node, you need to set nodeSelector.kubernetes.io/hostname to see the same result (one running and one pending pod). Otherwise, the pod can will be scheduled and running on the different node.

  Confirm the allocated address:

  ```bash
  # kubectl exec -n spyre-test2 pod0 -- cat /etc/aiu/senlib_config.json
  {"GENERAL":{"sen_bus_id":["0000:1a:00.0"],"target":"SOC"} ...
  ```

  Try deleting allocated pod:

  ```bash
  # kubectl delete po pod0 -n spyre-test2
  pod "pod0" deleted
  ```

  The pending pod should become running.

  ```bash
  # kubectl get po -n spyre-test2
  NAME   READY   STATUS    RESTARTS   AGE
  pod1   1/1     Running   0          63s
  ```

  Confirm the allocated address:

  ```bash
  # kubectl exec -n spyre-test2 pod1 -- cat /etc/aiu/senlib_config.json
  {"GENERAL":{"sen_bus_id":["0000:1a:00.0"],"target":"SOC"} ...
  ```

#### Example 3: specify a NUMA constraint

```yaml
    constraints:
    - requests: ["spyre"]
      matchAttribute: "spyre.ibm.com/numaInfo"
```

[Example](./demo/kind/spyre-test3.yaml)

```bash
# for kind
kubectl apply -f ./demo/kind/spyre-test3.yaml
# for OpenShift
kubectl apply -f ./demo/ocp/spyre-test3.yaml
```

The following result is expected result from the pseudo-topology.

Only two pod can be allocated. the remaining one pod should be pending.

```bash
  # kubectl get po -n spyre-test3
  NAME   READY   STATUS    RESTARTS   AGE
  pod0   1/1     Running   0          17s
  pod1   1/1     Running   0          16s
  pod2   0/1     Pending   0          16s
  # kubectl exec -n spyre-test3 pod0 -- cat /etc/aiu/senlib_config.json
  {"GENERAL":{"multi_aiu_config_path":"/tmp/testing","sen_bus_id":["0000:1e:00.0","0000:1a:00.0"],"target":"SOC"} ...
  # kubectl exec -n spyre-test3 pod1 -- cat /etc/aiu/senlib_config.json
  {"GENERAL":{"multi_aiu_config_path":"/tmp/testing","sen_bus_id":["0000:3d:00.0","0000:3f:00.0","0000:40:00.0"],"target":"SOC"} ...
```

> Note that pod0 and pod1 can get devices from the NUMAs differently from the above order.

After removal of pod1 (with 3 devices), the pod2 should become running.

```bash
  # kubectl delete po -n spyre-test3 pod1
  pod "pod1" deleted
  # kubectl get po -n spyre-test3
  NAME   READY   STATUS    RESTARTS   AGE
  pod0   1/1     Running   0          8m5s
  pod2   1/1     Running   0          8m4s
  # kubectl exec -n spyre-test3 pod2 -- cat /etc/aiu/senlib_config.json
  {"GENERAL":{"multi_aiu_config_path":"/tmp/testing","sen_bus_id":["0000:3d:00.0","0000:3f:00.0","0000:40:00.0"],"target":"SOC"} ...
  ```

### Clean up

```bash
make delete-cluster
```

## Development Guide

### Image build

```sh
make docker-build
```

> [!NOTE]
> The default docker image built is `docker.io/spyre-operator/dra-driver-spyre`.
> To push the image to remote registry, the remote registry must be set as follow:
>
>```sh
> REGISTRY=[your-remote-registry] make docker-build-push
>```

### Pre-commit hook

Pre-commit hooks help maintain code quality by running automated checks before each commit. The hooks are configured in `.pre-commit-config.yaml` and include:

- Code formatting (go-fmt, yamlfmt, shell-fmt)
- Linting (golangci-lint, codespell)
- Security checks (detect-secrets, detect-private-key)
- File validation (check-json, check-yaml, etc.)

#### Installation

Install pre-commit hooks:

```sh
pre-commit install --install-hooks
```

#### Manual execution

To run all hooks manually:

```sh
pre-commit run --all-files
```

#### Detect Secrets

The detect-secrets tool prevents secrets from being committed to the repository. It runs automatically as part of the pre-commit hook.

##### Install Detect Secrets CLI

To manually work with the `.secrets.baseline` file:

```sh
make detect-secrets-install
```

##### Scan for secrets

Create or update the `.secrets.baseline` file:

```sh
make secrets-scan
```

> [!NOTE]
> This command excludes `go.sum` files from scans.

##### Audit secrets

Review and classify detected secrets:

```sh
make secrets-audit
```

For each detected secret:
- Press `y` if it's an actual secret (then remove it from code and revoke credentials)
- Press `n` if it's a false positive

After addressing all secrets, manually update the `is_verified` field to `true` in `.secrets.baseline`.

##### Commit changes

After auditing, commit the updated `.secrets.baseline` file:

```sh
git add .secrets.baseline
git commit -m "Update secrets baseline"
```

Repeat the scan and audit process as needed when adding new code.
