# Metal3 Fake Kubernetes API server system (Metal3-FKAS)

## FKAS

Metal3-FKAS is a tool for testing CAPI-related projects.
When being asked, it generates new fake kubernetes api endpoint, that responds
to all typical requests that CAPI sends to a newly provisioned cluster.

Despite being developed for Metal3 ecosystem, FKAS is provider-free. It can be adopted
and used by any CAPI provider with intention to test the provider provisioning ability,
without using real nodes.

### Purpose

After a CAPI Infrastructure provisions a new cluster, CAPI will send queries
towards the newly launched cluster's API server to verify that the cluster is
fully up and running.

In a simulated provisioning process, there are no real nodes, hence we cannot
have an actual API server running inside the node. Booting up a real kubelet
and etcd servers elsewhere is possible, but these processes are likely to consume
a lot of resources.

FKAS is useful in this situation. When a request is sent towards `/register` endpoint,
it will spawn a new simulated kubernetes API server with *unique* a host and
port pair.
User can, then, inject the address into cluster template consumed by
CAPI with any infra provider.

### How to use

You can build the `metal3-fkas` image that is suitable for
your local environment with

```shell
make build-fkas
```

The result is an image with label `quay.io/metal3-io/metal3-fkas:<your-arch-name>`

Alternatively, you can also build a custom image with

```shell
cd hack/fake-apiserver
docker build -t <custom tag> .
```

For local tests, it's normally needed to load the image into the cluster.
For e.g. with `minikube`

```shell
minikube image load quay.io/metal3-io/metal3-fkas:latest
```

Now you can deploy this container to the cluster, for e.g. with the deployment
if k8s/metal3-fkas.yaml

```shell
kubectl apply -f metal3-fkas.yaml
```

After building the container image and deploy it to the bootstrap kubernetes cluster,
you need to create a tunnel to send request to it and get response, by using
a LoadBalancer, or a simple port-forward

```shell
fkas_pod_name=$(kubectl get pods -n default -l app=metal3-fkas-system -o jsonpath='{.items[0].metadata.name}')
kubectl port-forward pod/${fkas_pod_name} 3333:3333 2>/dev/null&
```

Now, you can generate a fake API server endpoint by sending
a POST request to the fake API server.

```shell
namespace=<cluster-namespace>
cluster_name=<cluster-name>

cluster_endpoint=$(curl -X POST "localhost:3333/register" \
-H "Content-Type: application/json" -d '{
  "cluster": "'$cluster_name'",
  "namespace": "'$namespace'"
}')
```

The fake API server will return a response with the ip and port of the newly
generated api server. For example:

```json
{
  "Resource": "metal3/test1",
  "Host": "10.244.0.83",
  "Port": 20000
}
```

A new cluster can be provisioned by injecting the host and port we
got from FKAS to the cluster template provided by a CAPI infrastructure
provider. For e.g., with CAPM3 that can be done as followed:

```shell
host=$(echo ${cluster_endpoints} | jq -r ".Host")
port=$(echo ${cluster_endpoints} | jq -r ".Port")

# Injecting the new api address into the cluster template by
# exporting these env vars
export CLUSTER_APIENDPOINT_HOST="${host}"
export CLUSTER_APIENDPOINT_PORT="${port}"

clusterctl generate cluster "${cluster}" \
  --from "${CLUSTER_TEMPLATE}" \
  --target-namespace "${namespace}" > /tmp/${cluster}-cluster.yaml
kubectl apply -f /tmp/${cluster}-cluster.yaml
```

After the cluster is created, CAPI will expect that information like node name
and provider ID is registered in the API server. Since our API server doesn't
live inside the node, we will need to feed the info to it, by sending a
PUT request to `/updateNode` endpoint:

```shell
curl -X PUT "localhost:3333/updateNode" -H "Content-Type: application/json" -d '{
  "cluster": "<cluster-name>",
  "namespace": "<namespace>",
  "nodeName": "<machine-object-name>",
  "providerID": "<provider-id>",
  "uuid": "<node-uuid>",
  "labels": "<node-labels>",
  "k8sversion": "<k8s-version-of-workload-cluster>"
}'
```

### Acknowledgements

This was developed thanks to the implementation of
[Cluster API Provider In Memory (CAPIM)](https://github.com/kubernetes-sigs/cluster-api/tree/main/test/infrastructure/inmemory).

## Metal3 FKAS System

### FKAS in Metal3

In metal3 ecosystem, currently we have two ways of simulating a workflow without
using any baremetal or virtual machines:

- [FakeIPA container](https://github.com/metal3-io/utility-images/tree/main/fake-ipa)
- BMO simulation mode

In both of these cases, the "nodes" are not able to boot up any kubernetes api server,
hence the needs of having mock API servers on-demands.

Similar to the general case, after having BMHs provisioned to `available` state,
the user can send a request towards the Fake API server endpoint `/register`,
which will spawn a new API server, with an unique `Host` and `Port` pair.

User can, then, use this IP address to feed the cluster template, by exporting
`CLUSTER_APIENDPOINT_HOST` and `CLUSTER_APIENDPOINT_PORT` variables.

There is no need of manually check and send node info to `/updateNode`, as we have
another tool to automate that part.

### Metal3-FKAS-Reconciler

This tool runs as a side-car container alongside FKAS, and works specifically
for Metal3. It eliminates the needs of user to manually fetch the nodes information
and send to `/updateNode` (as described earlier), by constantly watch the changes
in BMH objects, notice if a BMH is being provisioned to a kubernetes node, and
send request to `/updateNode` with appropriate information.

If you want to use *Metal3-FKAS* with another CAPI provider, you can also implement
your own reconciler, based on implementation of *metal3-fkas-reconciler*.

### Deployment

The `metal3-fkas-system` deployment (including `metal3-fkas` and `metal3-fkas-reconciler`)
can be deployed with the `k8s/metal3-fkas-system.yaml` file.

```shell
kubectl apply -f k8s/metal3-fkas-system.yaml
```

## Disclaimer

This is intended for development environments only.
Do **NOT** use it in production.
