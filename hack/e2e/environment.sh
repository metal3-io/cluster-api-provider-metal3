# File used as metal3-dev-env config and as a source for variables used by envsubst

# metal3-dev-env customization
export IMAGE_OS=${IMAGE_OS:-"Ubuntu"}
export CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-"docker"}
export EPHEMERAL_CLUSTER=${EPHEMERAL_CLUSTER:-"kind"}
export CAPI_VERSION=${CAPI_VERSION:-"v1alpha3"}
export CAPM3_VERSION=${CAPM3_VERSION:-"v1alpha4"}
export NUM_NODES="4"

# needed for variable substitution in templates
export IMAGE_FORMAT="raw"
export IMAGE_CHECKSUM_TYPE="md5"

# preserve the template, to be substituted based on value defined in e2e_test.go
# shellcheck disable=SC2016
export CONTROL_PLANE_MACHINE_COUNT='${CONTROL_PLANE_MACHINE_COUNT}'
# shellcheck disable=SC2016
export WORKER_MACHINE_COUNT='${WORKER_MACHINE_COUNT}'