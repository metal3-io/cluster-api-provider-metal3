# Fake API server

Fake API server is a tool running inside a kubernetes cluster,
and generates "fake" k8s api server endpoints on demand.

These endpoints respond to any request normally responded
by the apiserver of target clusters created in a normal CAPI
workflow. They can, hence, be used, for e.g., to represent the clusters
created by CAPM3 using fake hardware so that CAPI confirms that
the cluster was provisioned successfully.

## How to use

You can build the `fake-api-server` image that is suitable for
your local environment with

```shell
make build-fake-api-server
```

The result is an image with label `quay.io/metal3-io/fake-apiserver:<your-arch-name>`

Alternatively, you can also build a custom image with

```shell
cd hack/fake-apiserver
docker build -t <custom tag> .
```

For local tests, it's normally needed to load the image into the cluster.
For e.g. with `minikube`

```shell
docker image save -o /tmp/api-server.tar <image-name>
minikube image load /tmp/api-server.tar
```

After building the container image and deploy it to a kubernetes cluster,
you can generate a fake API server endpoint by sending
a GET request to the fake API server, with the following params:

```shell
"namespace": namespace in which the cluster will be created
"cluster_name": name of the cluster
"caKey": generated CA key
"caCert": generated CA cert
"etcdKey": generated etcd key
"etcdCert": generated etcd cert
```

```shell
curl <pod_ip>:<port>/register?resource=<namespace>/<cluster_name>&caKey=<caKeyEncoded>&caCert=<caCertEncoded>&etcdKey=<etcdKeyEncoded>&etcdCert=<etcdCertEncoded>"
```

The fake API server will return a response with the ip and port of the newly
generated api server. This can be fed to a CAPI infrastructure provider
(for e.g. CAPM3) to create a cluster.

After the cluster is created, information like node name and provider ID
can be added to the fake API server by sending a GET request to `/updateNode` endpoint:

```shell
<pod_ip>:<port>/updateNode?resource=<namespace>/<cluster_name>&nodeName=<node_name>&providerID=<providerID>"
```

## Acknowledgements

This was developed thanks to the implementation of
[Cluster API Provider In Memory (CAPIM)](https://github.com/kubernetes-sigs/cluster-api/tree/main/test/infrastructure/inmemory).

**NOTE:**:
This is intended for development environments only.
Do **not** use it in production.
