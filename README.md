# Spyre DRA Driver

This repo is initiated by https://github.com/kubernetes-sigs/dra-driver-spyre and https://github.com/NVIDIA/k8s-dra-driver.

## Use case

use case|available|example
---|---|---
Requesting a single Spyre card|:heavy_check_mark:|[Example 1](#example-1-general-request)
Requesting a specific Spyre card|:heavy_check_mark:|[Example 2](#example-2-specify-a-specific-pci-address)
Requesting 2 Spyre card within the same NUMA|:heavy_check_mark:|[Example 3](#example-3-specify-a-numa-constraint)

### Cluster

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

### Demo on Kind cluster

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

4. Deploy examples and check results

    > Must clean pods from each example before start testing the new example.

    ### Example 1: general request

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

    ### Example 2: specify a specific PCI address

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

  ### Example 3: specify a NUMA constraint

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

### Pre-commit hook

Detect Secrets tool will prevent your secrets from getting leaked. It is enabled by default as part of pre-commit hook, you just have to install it as below,

```sh
cd <repo path>
pre-commit install --install-hooks
```

With this, whenever changes are staged, detect-secrets hook will capture any leaked secret. However, if developer wants to scan repo on a routinely basis, follow the steps below:

```sh
cd <repo path>
pre-commit run detect-secrets --all-files
```

This will also catch any leaked-secret and fail the execution. If secrets are found, we have to scan and audit it as mentioned in following section.

#### Detect-secrets cli

This is required when you want to update `.secrets.baseline` file with regards to any new secret.

##### 1. Install Detect Secrets

```sh
cd <repo path>
make detect-secrets-install
```

##### 2. Perform secret scan

```sh
cd <repo path>
make secrets-scan
```

Note: running the above command will create a .secrets.baseline file or update if already exists. Currently `go.sum` files are excluded from the scans.

##### 3. Audit secrets

```sh
cd <repo path>
make secrets-audit
```

Indicate (y)es if the secret found is an actual secret or (n)o if it is a false positive.
If any secrets are found in your audit, remove them from the code, revoke the access of the credentials, commit your changes, and repeat step 3. Once you have verified that all secrets are addressed, update `is_verified` field for different secrets to `true` in `.secrets.baseline` manually.

##### 4. Commit .secrets.baseline and push commit

You can repeat steps 2 to 4 every time you want to scan secrets.
