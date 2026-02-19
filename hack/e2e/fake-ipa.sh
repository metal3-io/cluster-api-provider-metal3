#!/usr/bin/env bash

set -euxo pipefail

launch_fake_ipa()
{
    # Create a folder to host fakeIPA config and certs
    mkdir -p "${WORKING_DIR}/fake-ipa"
    if [[ "${EPHEMERAL_CLUSTER}" = "kind" ]] && [[ "${IRONIC_TLS_SETUP}" = "true" ]]; then
        cp "${IRONIC_CACERT_FILE}" "${WORKING_DIR}/fake-ipa/ironic-ca.crt"
    elif [[ "${IRONIC_TLS_SETUP}" = "true" ]]; then
        # wait for ironic to be running to ensure ironic-cert is created
        kubectl -n baremetal-operator-system wait --for=condition=available deployment/baremetal-operator-ironic --timeout=900s
        # Extract ironic-cert to be used inside fakeIPA for TLS
        kubectl get secret -n baremetal-operator-system ironic-cert -o json -o=jsonpath="{.data.ca\.crt}" | base64 -d > "${WORKING_DIR}/fake-ipa/ironic-ca.crt"
    fi

    # Create fake IPA custom config
    cat <<EOF > "${WORKING_DIR}/fake-ipa/config.py"
FAKE_IPA_API_URL = "https://${CLUSTER_BARE_METAL_PROVISIONER_IP}:${IRONIC_API_PORT}"
FAKE_IPA_INSPECTION_CALLBACK_URL = "${IRONIC_URL}/continue_inspection"
FAKE_IPA_ADVERTISE_ADDRESS_IP = "${EXTERNAL_SUBNET_V4_HOST}"
FAKE_IPA_INSECURE = ${FAKE_IPA_INSECURE:-False}
FAKE_IPA_CAFILE = "${FAKE_IPA_CAFILE:-/root/cert/ironic-ca.crt}"
FAKE_IPA_MIN_BOOT_TIME = ${FAKE_IPA_MIN_BOOT_TIME:-20}
FAKE_IPA_MAX_BOOT_TIME = ${FAKE_IPA_MAX_BOOT_TIME:-30}
EOF

    # shellcheck disable=SC2086
    sudo "${CONTAINER_RUNTIME}" run -d --net host --name fake-ipa ${POD_NAME_INFRA} \
        -v "${WORKING_DIR}/fake-ipa":/root/cert -v "/root/.ssh":/root/ssh \
        -e CONFIG='/root/cert/config.py' \
        "${FAKE_IPA_IMAGE}"
}
