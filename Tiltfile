# -*- mode: Python -*-

envsubst_cmd = "./hack/tools/bin/envsubst"

update_settings(k8s_upsert_timeout_secs=60)  # on first tilt up, often can take longer than 30 seconds

# set defaults
settings = {
    "allowed_contexts": [
        "kind-capm3"
    ],
    "deploy_cert_manager": True,
    "preload_images_for_kind": True,
    "kind_cluster_name": "capm3",
    "capi_version": "$CAPIRELEASE",
    "kubernetes_version": "$KUBERNETES_VERSION",
    "cert_manager_version": "v1.5.3",
    "enable_providers": [],
}

always_enable_providers = ["metal3"]
providers = {}
extra_args = settings.get("extra_args", {})

# global settings
settings.update(read_json(
    "tilt-settings.json",
    default = {},
))

if settings.get("trigger_mode") == "manual":
    trigger_mode(TRIGGER_MODE_MANUAL)

if "allowed_contexts" in settings:
    allow_k8s_contexts(settings.get("allowed_contexts"))

if "default_registry" in settings:
    default_registry(settings.get("default_registry"))

def load_provider_tiltfiles(provider_repos):
    for repo in provider_repos:
        file = repo + "/tilt-provider.json"
        provider_details = read_json(file, default = {})
        if type(provider_details) != type([]):
            provider_details = [provider_details]
        for item in provider_details:
            provider_name = item["name"]
            provider_config = item["config"]
            if "context" in provider_config:
                provider_config["context"] = repo + "/" + provider_config["context"]
            else:
                provider_config["context"] = repo
            if "kustomize_config" not in provider_config:
                provider_config["kustomize_config"] = True
            if "go_main" not in provider_config:
                provider_config["go_main"] = "main.go"
            providers[provider_name] = provider_config

# deploy CAPI
def deploy_capi():
    version = settings.get("capi_version")
    capi_uri = "https://github.com/kubernetes-sigs/cluster-api/releases/download/{}/cluster-api-components.yaml".format(version)
    cmd = "curl -sSL {} | {} | kubectl apply -f -".format(capi_uri, envsubst_cmd)
    local(cmd, quiet=True)
    if settings.get("extra_args"):
        extra_args = settings.get("extra_args")
        if extra_args.get("core"):
            core_extra_args = extra_args.get("core")
            if core_extra_args:
                for namespace in ["capi-system", "capi-webhook-system"]:
                    patch_args_with_extra_args(namespace, "capi-controller-manager", core_extra_args)
                patch_capi_manager_role_with_exp_infra_rbac()
        if extra_args.get("kubeadm-bootstrap"):
            kb_extra_args = extra_args.get("kubeadm-bootstrap")
            if kb_extra_args:
                patch_args_with_extra_args("capi-kubeadm-bootstrap-system", "capi-kubeadm-bootstrap-controller-manager", kb_extra_args)


def patch_args_with_extra_args(namespace, name, extra_args):
    args_str = str(local('kubectl get deployments {} -n {} -o jsonpath={{.spec.template.spec.containers[1].args}}'.format(name, namespace)))
    args_to_add = [arg for arg in extra_args if arg not in args_str]
    if args_to_add:
        args = args_str[1:-1].split()
        args.extend(args_to_add)
        patch = [{
            "op": "replace",
            "path": "/spec/template/spec/containers/1/args",
            "value": args,
        }]
        local("kubectl patch deployment {} -n {} --type json -p='{}'".format(name, namespace, str(encode_json(patch)).replace("\n", "")))


# patch the CAPI manager role to also provide access to experimental infrastructure
def patch_capi_manager_role_with_exp_infra_rbac():
    api_groups_str = str(local('kubectl get clusterrole capi-manager-role -o jsonpath={.rules[1].apiGroups}'))
    exp_infra_group = "exp.infrastructure.cluster.x-k8s.io"
    if exp_infra_group not in api_groups_str:
        groups = api_groups_str[1:-1].split() # "[arg1 arg2 ...]" trim off the first and last, then split
        groups.append(exp_infra_group)
        patch = [{
            "op": "replace",
            "path": "/rules/1/apiGroups",
            "value": groups,
        }]
        local("kubectl patch clusterrole capi-manager-role --type json -p='{}'".format(str(encode_json(patch)).replace("\n", "")))


# Users may define their own Tilt customizations in tilt.d. This directory is excluded from git and these files will
# not be checked in to version control.
def include_user_tilt_files():
    user_tiltfiles = listdir("tilt.d")
    for f in user_tiltfiles:
        include(f)


def append_arg_for_container_in_deployment(yaml_stream, name, namespace, contains_image_name, args):
    for item in yaml_stream:
        if item["kind"] == "Deployment" and item.get("metadata").get("name") == name and item.get("metadata").get("namespace") == namespace:
            containers = item.get("spec").get("template").get("spec").get("containers")
            for container in containers:
                if contains_image_name in container.get("image"):
                    container.get("args").extend(args)


def fixup_yaml_empty_arrays(yaml_str):
    yaml_str = yaml_str.replace("conditions: null", "conditions: []")
    return yaml_str.replace("storedVersions: null", "storedVersions: []")

tilt_helper_dockerfile_header = """
# Tilt image
FROM golang:1.17 as tilt-helper
# Support live reloading with Tilt
RUN wget --output-document /restart.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/restart.sh  && \
    wget --output-document /start.sh --quiet https://raw.githubusercontent.com/windmilleng/rerun-process-wrapper/master/start.sh && \
    chmod +x /start.sh && chmod +x /restart.sh
"""

tilt_dockerfile_header = """
FROM gcr.io/distroless/base:debug as tilt
WORKDIR /
COPY --from=tilt-helper /start.sh .
COPY --from=tilt-helper /restart.sh .
COPY manager .
"""

# Configures a provider by doing the following:
#
# 1. Enables a local_resource go build of the provider's manager binary
# 2. Configures a docker build for the provider, with live updating of the manager binary
# 3. Runs kustomize for the provider's config/ and applies it
def enable_provider(name):
    p = providers.get(name)

    name = p.get("name", name)
    context = p.get("context")
    go_main = p.get("go_main")
    label = p.get("label", name)

    # Prefix each live reload dependency with context. For example, for if the context is
    # test/infra/docker and main.go is listed as a dep, the result is test/infra/docker/main.go. This adjustment is
    # needed so Tilt can watch the correct paths for changes.
    live_reload_deps = []
    for d in p.get("live_reload_deps", []):
        live_reload_deps.append(context + "/" + d)

    # Set up a local_resource build of the provider's manager binary. The provider is expected to have a main.go in
    # manager_build_path or the main.go must be provided via go_main option. The binary is written to .tiltbuild/manager.
    local_resource(
        label.lower() + "_binary",
        cmd = "cd " + context + ';mkdir -p .tiltbuild;CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags \'-extldflags "-static"\' -o .tiltbuild/manager ' + go_main,
        deps = live_reload_deps,
        labels = [label, "ALL.binaries"],
    )

    additional_docker_helper_commands = p.get("additional_docker_helper_commands", "")
    additional_docker_build_commands = p.get("additional_docker_build_commands", "")

    dockerfile_contents = "\n".join([
        tilt_helper_dockerfile_header,
        additional_docker_helper_commands,
        tilt_dockerfile_header,
        additional_docker_build_commands,
    ])

    # Set up an image build for the provider. The live update configuration syncs the output from the local_resource
    # build into the container.
    entrypoint = ["sh", "/start.sh", "/manager"]
    provider_args = extra_args.get(name)
    if provider_args:
        entrypoint.extend(provider_args)

    docker_build(
        ref = p.get("image"),
        context = context + "/.tiltbuild/",
        dockerfile_contents = dockerfile_contents,
        target = "tilt",
        entrypoint = entrypoint,
        only = "manager",
        live_update = [
            sync(context + "/.tiltbuild/manager", "/manager"),
            run("sh /restart.sh"),
        ],
    )

    if p.get("kustomize_config"):

        # Copy all the substitutions from the user's tilt-settings.json into the environment. Otherwise, the substitutions
        # are not available and their placeholders will be replaced with the empty string when we call kustomize +
        # envsubst below.
        substitutions = settings.get("kustomize_substitutions", {})
        os.environ.update(substitutions)

        # Apply the kustomized yaml for this provider
        if name == "metal3-bmo":
          yaml = str(kustomizesub(context + "/config"))
        else:
          yaml = str(kustomizesub(context + "/config/default"))
        k8s_yaml(blob(yaml))
        get_controller_name(name)
    else:
        get_controller_name(name)

def get_controller_name(name):
    p = providers.get(name)
    name = p.get("name", name)
    label = p.get("label", name)
    manager_name = p.get("manager_name")
    if manager_name:
        k8s_resource(
            workload = manager_name,
            new_name = label.lower() + "_controller",
            labels = [label, "ALL.controllers"],
        )

# run worker clusters specified from 'tilt up' or in 'tilt_config.json'
def flavors():
    config.define_string_list("templates-to-run", args=True)
    config.define_string_list("worker-flavors")
    cfg = config.parse()
    worker_templates = cfg.get('templates-to-run', [])

    for flavor in cfg.get("worker-flavors", []):
        if flavor not in worker_templates:
            worker_templates.append(flavor)
    for flavor in worker_templates:
        deploy_worker_templates(flavor)

def deploy_worker_templates(flavor, substitutions):
    # validate flavor exists
    if flavor == "default":
        yaml_file = "./examples/clusterctl-templates/clusterctl-cluster.yaml"
    else:
        yaml_file = "./examples/clusterctl-templates/clusterctl-" + flavor + ".yaml"
        if not os.path.exists(yaml_file):
            fail(yaml_file + " not found")

    yaml = str(read_file(yaml_file))

    # substitutions to replace template variables
    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)

    # if metadata defined for worker-templates in tilt_settings
    if "worker-templates" in settings:
        # first priority replacements defined per template
        if "flavors" in settings.get("worker-templates", {}):
            substitutions = settings.get("worker-templates").get("flavors").get(flavor, {})
            for substitution in substitutions:
                value = substitutions[substitution]
                yaml = yaml.replace("${" + substitution + "}", value)

        # second priority replacements defined common to templates
        if "metadata" in settings.get("worker-templates", {}):
            substitutions = settings.get("worker-templates").get("metadata", {})
            for substitution in substitutions:
                value = substitutions[substitution]
                yaml = yaml.replace("${" + substitution + "}", value)

    # programmatically define any remaining vars
    substitutions = {
        "CLUSTER_NAME": flavor + "-template",
        "CONTROL_PLANE_MACHINE_COUNT": "1",
        "KUBERNETES_VERSION": settings.get("kubernetes_version"),
        "WORKER_MACHINE_COUNT": "1",
        "POD_CIDR": os.environ["POD_CIDR"],
        "SERVICE_CIDR": os.environ["SERVICE_CIDR"],
        "API_ENDPOINT_HOST": os.environ["API_ENDPOINT_HOST"],
        "API_ENDPOINT_PORT": os.environ["API_ENDPOINT_PORT"],
        "IMAGE_URL": os.environ["IMAGE_URL"],
        "IMAGE_CHECKSUM": os.environ["IMAGE_CHECKSUM"],
        "IMAGE_CHECKSUM_TYPE": os.environ["IMAGE_CHECKSUM_TYPE"],
        "IMAGE_FORMAT": os.environ["IMAGE_FORMAT"],
        "CTLPLANE_KUBEADM_EXTRA_CONFIG": os.environ["CTLPLANE_KUBEADM_EXTRA_CONFIG"],
        "WORKERS_KUBEADM_EXTRA_CONFIG": os.environ["WORKERS_KUBEADM_EXTRA_CONFIG"]
    }

    for substitution in substitutions:
        value = substitutions[substitution]
        yaml = yaml.replace("${" + substitution + "}", value)

    yaml = envsubst(yaml)
    yaml = yaml.replace('"', '\\"')     # add escape character to double quotes in yaml

    local_resource(
        "worker-" + flavor,
        cmd = "echo \"" + yaml + "\" > ./.tiltbuild/worker-" + flavor + ".yaml; cat ./.tiltbuild/worker-" + flavor + ".yaml | " + envsubst_cmd + " | kubectl apply -f -",
        auto_init = False,
        trigger_mode = TRIGGER_MODE_MANUAL
    )

# Enable core cluster-api plus everything listed in 'enable_providers' in tilt-settings.json
def enable_providers():
    local("make hack/tools/bin/kustomize hack/tools/bin/envsubst-drone")

    provider_repos = settings.get("provider_repos", [])
    union_provider_repos = [ k for k in provider_repos + ["."] ]
    load_provider_tiltfiles(union_provider_repos)

    user_enable_providers = settings.get("enable_providers", [])
    union_enable_providers = {k: "" for k in user_enable_providers + always_enable_providers}.keys()
    for name in union_enable_providers:
        enable_provider(name)

def base64_encode(to_encode):
    encode_blob = local("echo '{}' | tr -d '\n' | base64 - | tr -d '\n'".format(to_encode), quiet=True)
    return str(encode_blob)


def base64_encode_file(path_to_encode):
    encode_blob = local("cat {} | tr -d '\n' | base64 - | tr -d '\n'".format(path_to_encode), quiet=True)
    return str(encode_blob)


def base64_decode(to_decode):
    decode_blob = local("echo '{}' | base64 --decode -".format(to_decode), quiet=True)
    return str(decode_blob)

def envsubst(yaml):
    yaml = yaml.replace('"', '\\"')
    return str(local("echo \"{}\" | {}".format(yaml, envsubst_cmd), quiet=True))

def kustomizesub(folder):
    yaml = local('bash hack/kustomize-sub.sh {}'.format(folder), quiet=True)
    return yaml

##############################
# Actual work happens here
##############################

include_user_tilt_files()

load("ext://cert_manager", "deploy_cert_manager")

if settings.get("deploy_cert_manager"):
    deploy_cert_manager(version=settings.get("cert_manager_version"))

deploy_capi()

enable_providers()

flavors()
