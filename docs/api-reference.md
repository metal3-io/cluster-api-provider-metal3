# API Reference

Packages:

- [infrastructure.cluster.x-k8s.io/v1beta2](#infrastructureclusterx-k8siov1beta2)

# infrastructure.cluster.x-k8s.io/v1beta2

Resource Types:

- [Metal3Cluster](#metal3cluster)

- [Metal3ClusterTemplate](#metal3clustertemplate)

- [Metal3Machine](#metal3machine)

- [Metal3MachineTemplate](#metal3machinetemplate)

- [Metal3DataTemplate](#metal3datatemplate)

- [Metal3DataClaim](#metal3dataclaim)

- [Metal3Data](#metal3data)

- [Metal3Remediation](#metal3remediation)

- [Metal3RemediationTemplate](#metal3remediationtemplate)




## Metal3Cluster
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3Cluster is the Schema for the metal3clusters API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3Cluster</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3clusterspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3Cluster.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3Cluster.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.spec
<sup><sup>[↩ Parent](#metal3cluster)</sup></sup>



spec defines the desired state of Metal3Cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>cloudProviderEnabled</b></td>
        <td>boolean</td>
        <td>
          cloudProviderEnabled determines if the cluster is to be deployed with an external cloud provider.
If set to false, CAPM3 will use node labels to set providerID on the kubernetes nodes.
If set to true, providerID is set on nodes by other entities and CAPM3 uses the value of the providerID on the m3m resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterspeccontrolplaneendpoint">controlPlaneEndpoint</a></b></td>
        <td>object</td>
        <td>
          controlPlaneEndpoint represents the endpoint used to communicate with the control plane.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterspecfailuredomainsindex">failureDomains</a></b></td>
        <td>[]object</td>
        <td>
          failureDomains is a list of failure domain objects synced from the infrastructure provider.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.spec.controlPlaneEndpoint
<sup><sup>[↩ Parent](#metal3clusterspec)</sup></sup>



controlPlaneEndpoint represents the endpoint used to communicate with the control plane.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>host</b></td>
        <td>string</td>
        <td>
          host is the hostname on which the API server is serving.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          port is the port on which the API server is serving.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 1<br/>
            <i>Maximum</i>: 65535<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.spec.failureDomains[index]
<sup><sup>[↩ Parent](#metal3clusterspec)</sup></sup>



FailureDomain is the Schema for Cluster API failure domains.
It allows controllers to understand how many failure domains a cluster can optionally span across.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the failure domain.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>attributes</b></td>
        <td>map[string]string</td>
        <td>
          attributes is a free form map of attributes an infrastructure provider might use or require.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>controlPlane</b></td>
        <td>boolean</td>
        <td>
          controlPlane determines if this failure domain is suitable for use by control plane machines.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status
<sup><sup>[↩ Parent](#metal3cluster)</sup></sup>



status defines the observed state of Metal3Cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3clusterstatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          conditions defines current service state of the Metal3Cluster.
Known condition types are Ready, and Paused, BareMetalInfraStructureReady.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterstatusdeprecated">deprecated</a></b></td>
        <td>object</td>
        <td>
          deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterstatusfailuredomainsindex">failureDomains</a></b></td>
        <td>[]object</td>
        <td>
          failureDomains is a list of failure domain objects synced from the infrastructure provider.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clusterstatusinitialization">initialization</a></b></td>
        <td>object</td>
        <td>
          initialization provides observations of the Metal3Cluster initialization process.
NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdated</b></td>
        <td>string</td>
        <td>
          lastUpdated identifies when this status was last observed.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.conditions[index]
<sup><sup>[↩ Parent](#metal3clusterstatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.deprecated
<sup><sup>[↩ Parent](#metal3clusterstatus)</sup></sup>



deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3clusterstatusdeprecatedv1beta1">v1beta1</a></b></td>
        <td>object</td>
        <td>
          v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.deprecated.v1beta1
<sup><sup>[↩ Parent](#metal3clusterstatusdeprecated)</sup></sup>



v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3clusterstatusdeprecatedv1beta1conditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          conditions defines current service state of the Metal3Cluster.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureMessage</b></td>
        <td>string</td>
        <td>
          failureMessage indicates that there is a fatal problem reconciling the
state, and will be set to a descriptive error message.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureReason</b></td>
        <td>string</td>
        <td>
          failureReason indicates that there is a fatal problem reconciling the
state, and will be set to a token value suitable for
programmatic interpretation.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.deprecated.v1beta1.conditions[index]
<sup><sup>[↩ Parent](#metal3clusterstatusdeprecatedv1beta1)</sup></sup>



Condition defines an observation of a Cluster API resource operational state.

Deprecated: This type is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed. If that is not known, then using the time when
the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>string</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.
Many .condition.type values are consistent across resources like Available, but because arbitrary conditions
can be useful (see .node.status.conditions), the ability to deconflict is important.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This field may be empty.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason is the reason for the condition's last transition in CamelCase.
The specific API may choose whether or not this field is considered a guaranteed API.
This field may be empty.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>severity</b></td>
        <td>string</td>
        <td>
          severity provides an explicit classification of Reason code, so the users or machines can immediately
understand the current situation and act accordingly.
The Severity field MUST be set only when Status=False.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.failureDomains[index]
<sup><sup>[↩ Parent](#metal3clusterstatus)</sup></sup>



FailureDomain is the Schema for Cluster API failure domains.
It allows controllers to understand how many failure domains a cluster can optionally span across.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the failure domain.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>attributes</b></td>
        <td>map[string]string</td>
        <td>
          attributes is a free form map of attributes an infrastructure provider might use or require.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>controlPlane</b></td>
        <td>boolean</td>
        <td>
          controlPlane determines if this failure domain is suitable for use by control plane machines.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Cluster.status.initialization
<sup><sup>[↩ Parent](#metal3clusterstatus)</sup></sup>



initialization provides observations of the Metal3Cluster initialization process.
NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Cluster provisioning.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>provisioned</b></td>
        <td>boolean</td>
        <td>
          provisioned is true when the infrastructure provider reports that the Cluster's infrastructure is fully provisioned.
NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Cluster provisioning.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3ClusterTemplate
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3ClusterTemplate is the Schema for the metal3clustertemplates API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3ClusterTemplate</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3clustertemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3ClusterTemplate.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec
<sup><sup>[↩ Parent](#metal3clustertemplate)</sup></sup>



spec defines the desired state of Metal3ClusterTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3clustertemplatespectemplate">template</a></b></td>
        <td>object</td>
        <td>
          template describes the data for creating a Metal3Cluster from a template<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec.template
<sup><sup>[↩ Parent](#metal3clustertemplatespec)</sup></sup>



template describes the data for creating a Metal3Cluster from a template

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3clustertemplatespectemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3Cluster.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3clustertemplatespectemplatemetadata">metadata</a></b></td>
        <td>object</td>
        <td>
          metadata is the standard object's metadata.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec.template.spec
<sup><sup>[↩ Parent](#metal3clustertemplatespectemplate)</sup></sup>



spec defines the desired state of Metal3Cluster.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>cloudProviderEnabled</b></td>
        <td>boolean</td>
        <td>
          cloudProviderEnabled determines if the cluster is to be deployed with an external cloud provider.
If set to false, CAPM3 will use node labels to set providerID on the kubernetes nodes.
If set to true, providerID is set on nodes by other entities and CAPM3 uses the value of the providerID on the m3m resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clustertemplatespectemplatespeccontrolplaneendpoint">controlPlaneEndpoint</a></b></td>
        <td>object</td>
        <td>
          controlPlaneEndpoint represents the endpoint used to communicate with the control plane.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3clustertemplatespectemplatespecfailuredomainsindex">failureDomains</a></b></td>
        <td>[]object</td>
        <td>
          failureDomains is a list of failure domain objects synced from the infrastructure provider.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec.template.spec.controlPlaneEndpoint
<sup><sup>[↩ Parent](#metal3clustertemplatespectemplatespec)</sup></sup>



controlPlaneEndpoint represents the endpoint used to communicate with the control plane.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>host</b></td>
        <td>string</td>
        <td>
          host is the hostname on which the API server is serving.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>port</b></td>
        <td>integer</td>
        <td>
          port is the port on which the API server is serving.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 1<br/>
            <i>Maximum</i>: 65535<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec.template.spec.failureDomains[index]
<sup><sup>[↩ Parent](#metal3clustertemplatespectemplatespec)</sup></sup>



FailureDomain is the Schema for Cluster API failure domains.
It allows controllers to understand how many failure domains a cluster can optionally span across.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the failure domain.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>attributes</b></td>
        <td>map[string]string</td>
        <td>
          attributes is a free form map of attributes an infrastructure provider might use or require.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>controlPlane</b></td>
        <td>boolean</td>
        <td>
          controlPlane determines if this failure domain is suitable for use by control plane machines.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3ClusterTemplate.spec.template.metadata
<sup><sup>[↩ Parent](#metal3clustertemplatespectemplate)</sup></sup>



metadata is the standard object's metadata.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          annotations is an unstructured key value map stored with a resource that may be
set by external tools to store and retrieve arbitrary metadata. They are not
queryable and should be preserved when modifying objects.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          labels is a map of string keys and values that can be used to organize and categorize
(scope and select) objects. May match selectors of replication controllers
and services.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3Machine
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3Machine is the Schema for the metal3machines API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3Machine</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3machinespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3Machine.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3Machine.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec
<sup><sup>[↩ Parent](#metal3machine)</sup></sup>



spec defines the desired state of Metal3Machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>automatedCleaningMode</b></td>
        <td>enum</td>
        <td>
          automatedCleaningMode determines the mode of automated cleaning.
When set to disabled, automated cleaning of host disks will be skipped
during provisioning and deprovisioning.<br/>
          <br/>
            <i>Enum</i>: metadata, disabled<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespeccustomdeploy">customDeploy</a></b></td>
        <td>object</td>
        <td>
          customDeploy is a custom deploy procedure.
Either image or customDeploy is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespecdatatemplate">dataTemplate</a></b></td>
        <td>object</td>
        <td>
          dataTemplate is a reference to a Metal3DataTemplate object containing
a template of metadata to be rendered. Metadata keys defined in the
metadataTemplate take precedence over keys defined in metadata field.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureDomain</b></td>
        <td>string</td>
        <td>
          failureDomain is the failure domain unique identifier this machine should be attached to, as defined in Cluster API.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespechostselector">hostSelector</a></b></td>
        <td>object</td>
        <td>
          hostSelector specifies matching criteria for labels on BareMetalHosts.
This is used to limit the set of BareMetalHost objects considered for
claiming for a metal3machine.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespecimage">image</a></b></td>
        <td>object</td>
        <td>
          image is the image to be provisioned.
Either image or customDeploy is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespecmetadata">metaData</a></b></td>
        <td>object</td>
        <td>
          metaData is an object storing the reference to the secret containing the
metadata given by the user.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespecnetworkdata">networkData</a></b></td>
        <td>object</td>
        <td>
          networkData is an object storing the reference to the secret containing the
network data given by the user.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>providerID</b></td>
        <td>string</td>
        <td>
          providerID must match the provider ID as seen on the node object corresponding to this machine.
For Kubernetes Nodes running on the Metal3 provider, this value is set by the corresponding CPI component
and it has the format metal3://&lt;namespace&gt;/&lt;bmh-name&gt;/&lt;m3m-name&gt;.
The legacy format (metal3://&lt;bmh-uuid&gt;) will be deprecated in CAPM3 v1.13
and removed in CAPM3 v1.14.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinespecuserdata">userData</a></b></td>
        <td>object</td>
        <td>
          userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.customDeploy
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



customDeploy is a custom deploy procedure.
Either image or customDeploy is required.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>method</b></td>
        <td>string</td>
        <td>
          method is the name of the deploy method.
This name is specific to the deploy ramdisk used. If you don't have
a custom deploy ramdisk, you shouldn't use CustomDeploy.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.dataTemplate
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



dataTemplate is a reference to a Metal3DataTemplate object containing
a template of metadata to be rendered. Metadata keys defined in the
metadataTemplate take precedence over keys defined in metadata field.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.hostSelector
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



hostSelector specifies matching criteria for labels on BareMetalHosts.
This is used to limit the set of BareMetalHost objects considered for
claiming for a metal3machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinespechostselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions specifies match expressions that must be true on a chosen BareMetalHost<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels specifies key/value pairs of labels that must exist on a chosen BareMetalHost<br/>
          <br/>
            <i>Validations</i>:<li>self.all(k, size(self[k]) <= 63): matchLabels values at most 63 characters</li>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.hostSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#metal3machinespechostselector)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string of required values.
An empty list is valid and required for operators that take no values,
such as "exists" and "!". Field presence is therefore not required: an
empty slice is dropped by omitempty, and a nil slice would marshal to
null and be pruned, so requiring presence would reject valid selectors.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.image
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



image is the image to be provisioned.
Either image or customDeploy is required.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          url is a location of an image to deploy.<br/>
          <br/>
            <i>Format</i>: uri<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          checksum is a md5sum, sha256sum or sha512sum value or a URL to retrieve one.
Optional for live-iso and oci:// URLs; required otherwise.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>checksumType</b></td>
        <td>enum</td>
        <td>
          checksumType is the checksum algorithm for the image.
e.g md5, sha256, sha512<br/>
          <br/>
            <i>Enum</i>: md5, sha256, sha512<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>diskFormat</b></td>
        <td>enum</td>
        <td>
          diskFormat contains the image disk format.<br/>
          <br/>
            <i>Enum</i>: raw, qcow2, vdi, vmdk, live-iso<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.metaData
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



metaData is an object storing the reference to the secret containing the
metadata given by the user.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.networkData
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



networkData is an object storing the reference to the secret containing the
network data given by the user.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.spec.userData
<sup><sup>[↩ Parent](#metal3machinespec)</sup></sup>



userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status
<sup><sup>[↩ Parent](#metal3machine)</sup></sup>



status defines the observed state of Metal3Machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinestatusaddressesindex">addresses</a></b></td>
        <td>[]object</td>
        <td>
          addresses contains the associated addresses for the machine.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusconditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          conditions represents the observations of a Metal3Machine's current state.
Known condition types are Ready, AssociateBareMetalHost, AssociateMetal3MachineMetaData, Metal3DataReady and Paused.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusdeprecated">deprecated</a></b></td>
        <td>object</td>
        <td>
          deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusinitialization">initialization</a></b></td>
        <td>object</td>
        <td>
          initialization provides observations of the Metal3Machine initialization process.
NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdated</b></td>
        <td>string</td>
        <td>
          lastUpdated identifies when this status was last observed.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusmetadata">metaData</a></b></td>
        <td>object</td>
        <td>
          metaData is an object storing the reference to the secret containing the
metadata used to deploy the BareMetalHost.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusnetworkdata">networkData</a></b></td>
        <td>object</td>
        <td>
          networkData is an object storing the reference to the secret containing the
network data used to deploy the BareMetalHost.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatusrendereddata">renderedData</a></b></td>
        <td>object</td>
        <td>
          renderedData is a reference to a rendered Metal3Data object containing
the references to metaData and networkData secrets.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinestatususerdata">userData</a></b></td>
        <td>object</td>
        <td>
          userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.addresses[index]
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



MachineAddress contains information for the node's address.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>address</b></td>
        <td>string</td>
        <td>
          address is the machine address.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          type is the machine address type, one of Hostname, ExternalIP, InternalIP, ExternalDNS or InternalDNS.<br/>
          <br/>
            <i>Enum</i>: Hostname, ExternalIP, InternalIP, ExternalDNS, InternalDNS<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3Machine.status.conditions[index]
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



Condition contains details for one aspect of the current state of this API Resource.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This may be an empty string.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason contains a programmatic identifier indicating the reason for the condition's last transition.
Producers of specific condition types may define expected values and meanings for this field,
and whether the values are considered a guaranteed API.
The value should be a CamelCase string.
This field may not be empty.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>enum</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
          <br/>
            <i>Enum</i>: True, False, Unknown<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>observedGeneration</b></td>
        <td>integer</td>
        <td>
          observedGeneration represents the .metadata.generation that the condition was set based upon.
For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
with respect to the current state of the instance.<br/>
          <br/>
            <i>Format</i>: int64<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.deprecated
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



deprecated groups all the status fields that are deprecated and will be removed when all the nested field are removed.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinestatusdeprecatedv1beta1">v1beta1</a></b></td>
        <td>object</td>
        <td>
          v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.deprecated.v1beta1
<sup><sup>[↩ Parent](#metal3machinestatusdeprecated)</sup></sup>



v1beta1 groups all the status fields that are deprecated and will be removed when support for v1beta1 will be dropped.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinestatusdeprecatedv1beta1conditionsindex">conditions</a></b></td>
        <td>[]object</td>
        <td>
          conditions defines current service state of the Metal3Machine.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureMessage</b></td>
        <td>string</td>
        <td>
          failureMessage will be set in the event that there is a terminal problem
reconciling the metal3machine and will contain a more verbose string suitable
for logging and human consumption.

This field should not be set for transitive errors that a controller
faces that are expected to be fixed automatically over
time (like service outages), but instead indicate that something is
fundamentally wrong with the metal3machine's spec or the configuration of
the controller, and that manual intervention is required. Examples
of terminal errors would be invalid combinations of settings in the
spec, values that are unsupported by the controller, or the
responsible controller itself being critically misconfigured.

Any transient errors that occur during the reconciliation of
metal3machines can be added as events to the metal3machine object
and/or logged in the controller's output.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureReason</b></td>
        <td>string</td>
        <td>
          failureReason will be set in the event that there is a terminal problem
reconciling the metal3machine and will contain a succinct value suitable
for machine interpretation.

This field should not be set for transitive errors that a controller
faces that are expected to be fixed automatically over
time (like service outages), but instead indicate that something is
fundamentally wrong with the metal3machine's spec or the configuration of
the controller, and that manual intervention is required. Examples
of terminal errors would be invalid combinations of settings in the
spec, values that are unsupported by the controller, or the
responsible controller itself being critically misconfigured.

Any transient errors that occur during the reconciliation of
metal3machines can be added as events to the metal3machine object
and/or logged in the controller's output.

Deprecated: This field is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.deprecated.v1beta1.conditions[index]
<sup><sup>[↩ Parent](#metal3machinestatusdeprecatedv1beta1)</sup></sup>



Condition defines an observation of a Cluster API resource operational state.

Deprecated: This type is deprecated and is going to be removed when support for v1beta1 will be dropped. Please see https://github.com/kubernetes-sigs/cluster-api/blob/main/docs/proposals/20240916-improve-status-in-CAPI-resources.md for more details.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastTransitionTime</b></td>
        <td>string</td>
        <td>
          lastTransitionTime is the last time the condition transitioned from one status to another.
This should be when the underlying condition changed. If that is not known, then using the time when
the API field changed is acceptable.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>string</td>
        <td>
          status of the condition, one of True, False, Unknown.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>string</td>
        <td>
          type of condition in CamelCase or in foo.example.com/CamelCase.
Many .condition.type values are consistent across resources like Available, but because arbitrary conditions
can be useful (see .node.status.conditions), the ability to deconflict is important.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>message</b></td>
        <td>string</td>
        <td>
          message is a human readable message indicating details about the transition.
This field may be empty.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>reason</b></td>
        <td>string</td>
        <td>
          reason is the reason for the condition's last transition in CamelCase.
The specific API may choose whether or not this field is considered a guaranteed API.
This field may be empty.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>severity</b></td>
        <td>string</td>
        <td>
          severity provides an explicit classification of Reason code, so the users or machines can immediately
understand the current situation and act accordingly.
The Severity field MUST be set only when Status=False.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.initialization
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



initialization provides observations of the Metal3Machine initialization process.
NOTE: Fields in this struct are part of the Cluster API contract and are used to orchestrate initial Machine provisioning.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>provisioned</b></td>
        <td>boolean</td>
        <td>
          provisioned is true when the infrastructure provider reports that the Machine's infrastructure is fully provisioned.
NOTE: this field is part of the Cluster API contract, and it is used to orchestrate initial Machine provisioning.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.metaData
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



metaData is an object storing the reference to the secret containing the
metadata used to deploy the BareMetalHost.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.networkData
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



networkData is an object storing the reference to the secret containing the
network data used to deploy the BareMetalHost.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.renderedData
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



renderedData is a reference to a rendered Metal3Data object containing
the references to metaData and networkData secrets.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Machine.status.userData
<sup><sup>[↩ Parent](#metal3machinestatus)</sup></sup>



userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3MachineTemplate
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3MachineTemplate is the Schema for the metal3machinetemplates API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3MachineTemplate</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3MachineTemplate.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec
<sup><sup>[↩ Parent](#metal3machinetemplate)</sup></sup>



spec defines the desired state of Metal3MachineTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinetemplatespectemplate">template</a></b></td>
        <td>object</td>
        <td>
          template describes the data needed to create a Metal3Machine from a template<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>nodeReuse</b></td>
        <td>boolean</td>
        <td>
          nodeReuse is a flag that can be set to True to enable node reuse during upgrade.
When set to True, CAPM3 Machine controller will
pick the same pool of BMHs' that were released during the upgrade operation.<br/>
          <br/>
            <i>Default</i>: false<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template
<sup><sup>[↩ Parent](#metal3machinetemplatespec)</sup></sup>



template describes the data needed to create a Metal3Machine from a template

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec is the specification of the desired behavior of the machine.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatemetadata">metadata</a></b></td>
        <td>object</td>
        <td>
          metadata is the standard object's metadata.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplate)</sup></sup>



spec is the specification of the desired behavior of the machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>automatedCleaningMode</b></td>
        <td>enum</td>
        <td>
          automatedCleaningMode determines the mode of automated cleaning.
When set to disabled, automated cleaning of host disks will be skipped
during provisioning and deprovisioning.<br/>
          <br/>
            <i>Enum</i>: metadata, disabled<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespeccustomdeploy">customDeploy</a></b></td>
        <td>object</td>
        <td>
          customDeploy is a custom deploy procedure.
Either image or customDeploy is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespecdatatemplate">dataTemplate</a></b></td>
        <td>object</td>
        <td>
          dataTemplate is a reference to a Metal3DataTemplate object containing
a template of metadata to be rendered. Metadata keys defined in the
metadataTemplate take precedence over keys defined in metadata field.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>failureDomain</b></td>
        <td>string</td>
        <td>
          failureDomain is the failure domain unique identifier this machine should be attached to, as defined in Cluster API.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespechostselector">hostSelector</a></b></td>
        <td>object</td>
        <td>
          hostSelector specifies matching criteria for labels on BareMetalHosts.
This is used to limit the set of BareMetalHost objects considered for
claiming for a metal3machine.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespecimage">image</a></b></td>
        <td>object</td>
        <td>
          image is the image to be provisioned.
Either image or customDeploy is required.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespecmetadata">metaData</a></b></td>
        <td>object</td>
        <td>
          metaData is an object storing the reference to the secret containing the
metadata given by the user.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespecnetworkdata">networkData</a></b></td>
        <td>object</td>
        <td>
          networkData is an object storing the reference to the secret containing the
network data given by the user.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>providerID</b></td>
        <td>string</td>
        <td>
          providerID must match the provider ID as seen on the node object corresponding to this machine.
For Kubernetes Nodes running on the Metal3 provider, this value is set by the corresponding CPI component
and it has the format metal3://&lt;namespace&gt;/&lt;bmh-name&gt;/&lt;m3m-name&gt;.
The legacy format (metal3://&lt;bmh-uuid&gt;) will be deprecated in CAPM3 v1.13
and removed in CAPM3 v1.14.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespecuserdata">userData</a></b></td>
        <td>object</td>
        <td>
          userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.customDeploy
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



customDeploy is a custom deploy procedure.
Either image or customDeploy is required.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>method</b></td>
        <td>string</td>
        <td>
          method is the name of the deploy method.
This name is specific to the deploy ramdisk used. If you don't have
a custom deploy ramdisk, you shouldn't use CustomDeploy.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.dataTemplate
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



dataTemplate is a reference to a Metal3DataTemplate object containing
a template of metadata to be rendered. Metadata keys defined in the
metadataTemplate take precedence over keys defined in metadata field.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.hostSelector
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



hostSelector specifies matching criteria for labels on BareMetalHosts.
This is used to limit the set of BareMetalHost objects considered for
claiming for a metal3machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3machinetemplatespectemplatespechostselectormatchexpressionsindex">matchExpressions</a></b></td>
        <td>[]object</td>
        <td>
          matchExpressions specifies match expressions that must be true on a chosen BareMetalHost<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>matchLabels</b></td>
        <td>map[string]string</td>
        <td>
          matchLabels specifies key/value pairs of labels that must exist on a chosen BareMetalHost<br/>
          <br/>
            <i>Validations</i>:<li>self.all(k, size(self[k]) <= 63): matchLabels values at most 63 characters</li>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.hostSelector.matchExpressions[index]
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespechostselector)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key is the label key that the selector applies to.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>operator</b></td>
        <td>string</td>
        <td>
          operator represents a key's relationship to a set of values.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>values</b></td>
        <td>[]string</td>
        <td>
          values is an array of string of required values.
An empty list is valid and required for operators that take no values,
such as "exists" and "!". Field presence is therefore not required: an
empty slice is dropped by omitempty, and a nil slice would marshal to
null and be pruned, so requiring presence would reject valid selectors.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.image
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



image is the image to be provisioned.
Either image or customDeploy is required.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>url</b></td>
        <td>string</td>
        <td>
          url is a location of an image to deploy.<br/>
          <br/>
            <i>Format</i>: uri<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>checksum</b></td>
        <td>string</td>
        <td>
          checksum is a md5sum, sha256sum or sha512sum value or a URL to retrieve one.
Optional for live-iso and oci:// URLs; required otherwise.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>checksumType</b></td>
        <td>enum</td>
        <td>
          checksumType is the checksum algorithm for the image.
e.g md5, sha256, sha512<br/>
          <br/>
            <i>Enum</i>: md5, sha256, sha512<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>diskFormat</b></td>
        <td>enum</td>
        <td>
          diskFormat contains the image disk format.<br/>
          <br/>
            <i>Enum</i>: raw, qcow2, vdi, vmdk, live-iso<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.metaData
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



metaData is an object storing the reference to the secret containing the
metadata given by the user.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.networkData
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



networkData is an object storing the reference to the secret containing the
network data given by the user.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.spec.userData
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplatespec)</sup></sup>



userData references the Secret that holds user data needed by the bare metal
operator. The Namespace is optional; it will default to the metal3machine's
namespace if not specified.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3MachineTemplate.spec.template.metadata
<sup><sup>[↩ Parent](#metal3machinetemplatespectemplate)</sup></sup>



metadata is the standard object's metadata.
More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>
          annotations is an unstructured key value map stored with a resource that may be
set by external tools to store and retrieve arbitrary metadata. They are not
queryable and should be preserved when modifying objects.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>
          labels is a map of string keys and values that can be used to organize and categorize
(scope and select) objects. May match selectors of replication controllers
and services.
More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3DataTemplate
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3DataTemplate is the Schema for the metal3datatemplates API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3DataTemplate</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3DataTemplate.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatestatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3DataTemplate.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec
<sup><sup>[↩ Parent](#metal3datatemplate)</sup></sup>



spec defines the desired state of Metal3DataTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>clusterName</b></td>
        <td>string</td>
        <td>
          clusterName is the name of the Cluster this object belongs to.

Deprecated: This field is deprecated and will be removed in a future release.
It is no longer used by the controllers.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadata">metaData</a></b></td>
        <td>object</td>
        <td>
          metaData contains the information needed to generate the metadata secret<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdata">networkData</a></b></td>
        <td>object</td>
        <td>
          networkData contains the information needed to generate the networkdata
secret<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData
<sup><sup>[↩ Parent](#metal3datatemplatespec)</sup></sup>



metaData contains the information needed to generate the metadata secret

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecmetadatadnsserversfrompoolindex">dnsServersFromPool</a></b></td>
        <td>[]object</td>
        <td>
          dnsServersFromPool is the list of metadata items to be rendered as dns servers.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatafromannotationsindex">fromAnnotations</a></b></td>
        <td>[]object</td>
        <td>
          fromAnnotations is the list of metadata items to be fetched from object
Annotations<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatafromhostinterfacesindex">fromHostInterfaces</a></b></td>
        <td>[]object</td>
        <td>
          fromHostInterfaces is the list of metadata items to be rendered as MAC
addresses of the host interfaces.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatafromlabelsindex">fromLabels</a></b></td>
        <td>[]object</td>
        <td>
          fromLabels is the list of metadata items to be fetched from object labels<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatagatewaysfrompoolindex">gatewaysFromPool</a></b></td>
        <td>[]object</td>
        <td>
          gatewaysFromPool is the list of metadata items to be rendered as gateway addresses.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadataindexesindex">indexes</a></b></td>
        <td>[]object</td>
        <td>
          indexes is the list of metadata items to be rendered from the index of the
Metal3Data<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadataipaddressesfrompoolindex">ipAddressesFromPool</a></b></td>
        <td>[]object</td>
        <td>
          ipAddressesFromPool is the list of metadata items to be rendered as ip addresses.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatanamespacesindex">namespaces</a></b></td>
        <td>[]object</td>
        <td>
          namespaces is the list of metadata items to be rendered from the namespace<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadataobjectnamesindex">objectNames</a></b></td>
        <td>[]object</td>
        <td>
          objectNames is the list of metadata items to be rendered from the name
of objects.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadataprefixesfrompoolindex">prefixesFromPool</a></b></td>
        <td>[]object</td>
        <td>
          prefixesFromPool is the list of metadata items to be rendered as network prefixes.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecmetadatastringsindex">strings</a></b></td>
        <td>[]object</td>
        <td>
          strings is the list of metadata items to be rendered from strings<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.dnsServersFromPool[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the IP pool used to fetch the value to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup is the api group of the IP pool.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind is the kind of the IP pool<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.fromAnnotations[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataFromAnnotation contains the information to fetch an annotation
content, if the label does not exist, it is rendered as empty string.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation to fetch<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.fromHostInterfaces[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataHostInterface contains the information to render the object name.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>fromBootMAC</b></td>
        <td>boolean</td>
        <td>
          fromBootMAC will fetch the MAC address from the BareMetalHost Spec BootMACAddress field.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>interface</b></td>
        <td>string</td>
        <td>
          interface is the name of the interface in the BareMetalHost Status Hardware
Details list of interfaces from which to fetch the MAC address.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.fromLabels[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataFromLabel contains the information to fetch a label content, if the
label does not exist, it is rendered as empty string.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>label</b></td>
        <td>string</td>
        <td>
          label is the key of the label to fetch<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.gatewaysFromPool[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the IP pool used to fetch the value to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup is the api group of the IP pool.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind is the kind of the IP pool<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.indexes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataIndex contains the information to render the index.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>offset</b></td>
        <td>integer</td>
        <td>
          offset is the offset to apply to the index when rendering it<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>string</td>
        <td>
          prefix is the prefix string<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>step</b></td>
        <td>integer</td>
        <td>
          step is the multiplier of the index<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>suffix</b></td>
        <td>string</td>
        <td>
          suffix is the suffix string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.ipAddressesFromPool[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the IP pool used to fetch the value to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup is the api group of the IP pool.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind is the kind of the IP pool<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.namespaces[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataNamespace contains the information to render the namespace.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.objectNames[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataObjectName contains the information to render the object name.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.prefixesFromPool[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>





<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the IP pool used to fetch the value to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup is the api group of the IP pool.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind is the kind of the IP pool<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.metaData.strings[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecmetadata)</sup></sup>



MetaDataString contains the information to render the string.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>key</b></td>
        <td>string</td>
        <td>
          key will be used as the key to set in the metadata map for cloud-init<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>string</td>
        <td>
          value is the string to render.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData
<sup><sup>[↩ Parent](#metal3datatemplatespec)</sup></sup>



networkData contains the information needed to generate the networkdata
secret

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinks">links</a></b></td>
        <td>object</td>
        <td>
          links is a structure containing lists of different types objects<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworks">networks</a></b></td>
        <td>object</td>
        <td>
          networks  is a structure containing lists of different types objects<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdataservices">services</a></b></td>
        <td>object</td>
        <td>
          services  is a structure containing lists of different types objects<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdata)</sup></sup>



links is a structure containing lists of different types objects

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksbondsindex">bonds</a></b></td>
        <td>[]object</td>
        <td>
          bonds contains a list of Bond links<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksethernetsindex">ethernets</a></b></td>
        <td>[]object</td>
        <td>
          ethernets contains a list of Ethernet links<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksvlansindex">vlans</a></b></td>
        <td>[]object</td>
        <td>
          vlans contains a list of Vlan links<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.bonds[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinks)</sup></sup>



NetworkDataLinkBond represents a bond link object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>bondMode</b></td>
        <td>enum</td>
        <td>
          bondMode is the mode of bond used. It can be one of
balance-rr, active-backup, balance-xor, broadcast, balance-tlb, balance-alb, 802.3ad<br/>
          <br/>
            <i>Enum</i>: balance-rr, active-backup, balance-xor, broadcast, balance-tlb, balance-alb, 802.3ad<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the ID of the interface (used for naming)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksbondsindexmacaddress">macAddress</a></b></td>
        <td>object</td>
        <td>
          macAddress is the MAC address of the interface, containing the object
used to render it.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>bondLinks</b></td>
        <td>[]string</td>
        <td>
          bondLinks is the list of links that are part of the bond.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>bondXmitHashPolicy</b></td>
        <td>enum</td>
        <td>
          bondXmitHashPolicy selects the transmit hash policy used for port selection in balance-xor and 802.3ad modes<br/>
          <br/>
            <i>Enum</i>: layer2, layer3+4, layer2+3<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>mtu</b></td>
        <td>integer</td>
        <td>
          mtu is the MTU of the interface<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1500<br/>
            <i>Minimum</i>: 1280<br/>
            <i>Maximum</i>: 9000<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the interface name to be used by cloud-init. When combined with
macAddress, cloud-init will rename the interface matching the MAC to this name.
When macAddress is omitted, cloud-init will use this name directly.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksbondsindexparametersindex">parameters</a></b></td>
        <td>[]object</td>
        <td>
          parameters blob passed without any validation/modifications into cloud-init config
Bond parameters are optional cloud-init extras, and were optional in
v1beta1, so neither field presence nor a minimum item count can be
required without rejecting existing bond links that set none.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.bonds[index].macAddress
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksbondsindex)</sup></sup>



macAddress is the MAC address of the interface, containing the object
used to render it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksbondsindexmacaddressfromannotation">fromAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromAnnotation references an object annotation to retrieve the
MAC address from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fromHostInterface</b></td>
        <td>string</td>
        <td>
          fromHostInterface contains the name of the interface in the BareMetalHost
Introspection details from which to fetch the MAC address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string contains the MAC address given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.bonds[index].macAddress.fromAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksbondsindexmacaddress)</sup></sup>



fromAnnotation references an object annotation to retrieve the
MAC address from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation to fetch<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.bonds[index].parameters[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksbondsindex)</sup></sup>



NetworkDataLinkBondParam represents a single bond parameter.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the parameter name<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>value</b></td>
        <td>JSON</td>
        <td>
          value is the parameter value<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.ethernets[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinks)</sup></sup>



NetworkDataLinkEthernet represents an ethernet link object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the ID of the interface (used for naming)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksethernetsindexmacaddress">macAddress</a></b></td>
        <td>object</td>
        <td>
          macAddress is the MAC address of the interface, containing the object
used to render it.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          type is the type of the ethernet link. It can be one of:
bridge, dvs, hw_veb, hyperv, ovs, tap, vhostuser, vif, phy<br/>
          <br/>
            <i>Enum</i>: bridge, dvs, hw_veb, hyperv, ovs, tap, vhostuser, vif, phy<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>mtu</b></td>
        <td>integer</td>
        <td>
          mtu is the MTU of the interface<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1500<br/>
            <i>Minimum</i>: 1280<br/>
            <i>Maximum</i>: 9000<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the interface name to be used by cloud-init. When combined with
MACAddress, cloud-init will rename the interface matching the MAC to this name.
When MACAddress is omitted, cloud-init will use this name directly.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.ethernets[index].macAddress
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksethernetsindex)</sup></sup>



macAddress is the MAC address of the interface, containing the object
used to render it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksethernetsindexmacaddressfromannotation">fromAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromAnnotation references an object annotation to retrieve the
MAC address from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fromHostInterface</b></td>
        <td>string</td>
        <td>
          fromHostInterface contains the name of the interface in the BareMetalHost
Introspection details from which to fetch the MAC address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string contains the MAC address given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.ethernets[index].macAddress.fromAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksethernetsindexmacaddress)</sup></sup>



fromAnnotation references an object annotation to retrieve the
MAC address from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation to fetch<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.vlans[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinks)</sup></sup>



NetworkDataLinkVlan represents a vlan link object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the ID of the interface (used for naming)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksvlansindexmacaddress">macAddress</a></b></td>
        <td>object</td>
        <td>
          macAddress is the MAC address of the interface, containing the object
used to render it.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>vlanID</b></td>
        <td>integer</td>
        <td>
          vlanID is the Vlan ID<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 4096<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>vlanLink</b></td>
        <td>string</td>
        <td>
          vlanLink is the name of the link on which the vlan should be added<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>mtu</b></td>
        <td>integer</td>
        <td>
          mtu is the MTU of the interface<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1500<br/>
            <i>Minimum</i>: 1280<br/>
            <i>Maximum</i>: 9000<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the interface name to be used by cloud-init. When combined with
macAddress, cloud-init will rename the interface matching the MAC to this name.
When macAddress is omitted, cloud-init will use this name directly.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.vlans[index].macAddress
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksvlansindex)</sup></sup>



macAddress is the MAC address of the interface, containing the object
used to render it.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatalinksvlansindexmacaddressfromannotation">fromAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromAnnotation references an object annotation to retrieve the
MAC address from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>fromHostInterface</b></td>
        <td>string</td>
        <td>
          fromHostInterface contains the name of the interface in the BareMetalHost
Introspection details from which to fetch the MAC address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string contains the MAC address given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.links.vlans[index].macAddress.fromAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatalinksvlansindexmacaddress)</sup></sup>



fromAnnotation references an object annotation to retrieve the
MAC address from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation to fetch<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the name<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdata)</sup></sup>



networks  is a structure containing lists of different types objects

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4index">ipv4</a></b></td>
        <td>[]object</td>
        <td>
          ipv4 contains a list of IPv4 static allocations<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindex">ipv4DHCP</a></b></td>
        <td>[]object</td>
        <td>
          ipv4DHCP contains a list of IPv4 DHCP allocations<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6index">ipv6</a></b></td>
        <td>[]object</td>
        <td>
          ipv6 contains a list of IPv6 static allocations<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindex">ipv6DHCP</a></b></td>
        <td>[]object</td>
        <td>
          ipv6DHCP contains a list of IPv6 DHCP allocations<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindex">ipv6SLAAC</a></b></td>
        <td>[]object</td>
        <td>
          ipv6SLAAC contains a list of IPv6 SLAAC allocations<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworks)</sup></sup>



NetworkDataIPv4 represents an ipv4 static network object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the network ID (name)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>link</b></td>
        <td>string</td>
        <td>
          link is the link on which the network applies<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool reference via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, ipAddressFromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to allocate an address from.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ipAddressFromIPPool</b></td>
        <td>string</td>
        <td>
          ipAddressFromIPPool contains the name of the IP pool to use to get an ip address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindex">routes</a></b></td>
        <td>[]object</td>
        <td>
          routes contains a list of IPv4 routes<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4index)</sup></sup>



fromPoolAnnotation allows specifying the pool reference via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, ipAddressFromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4index)</sup></sup>



fromPoolRef is a reference to a IP pool to allocate an address from.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].routes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4index)</sup></sup>



NetworkDataRoutev4 represents an ipv4 route object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexgateway">gateway</a></b></td>
        <td>object</td>
        <td>
          gateway is the IPv4 address of the gateway<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>network</b></td>
        <td>string</td>
        <td>
          network is the IPv4 network address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          prefix is the mask of the network as integer (max 32)<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexservices">services</a></b></td>
        <td>object</td>
        <td>
          services is a list of IPv4 services<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].routes[index].gateway
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindex)</sup></sup>



gateway is the IPv4 address of the gateway

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fromIPPool</b></td>
        <td>string</td>
        <td>
          fromIPPool is the name of the IPPool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexgatewayfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexgatewayfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string is the gateway given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].routes[index].gateway.fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexgateway)</sup></sup>



fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].routes[index].gateway.fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindexgateway)</sup></sup>



fromPoolRef is a reference to a IP pool to fetch the gateway from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4[index].routes[index].services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4indexroutesindex)</sup></sup>



services is a list of IPv4 services

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of IPv4 DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworks)</sup></sup>



NetworkDataIPv4DHCP represents an ipv4 DHCP network object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the network ID (name)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>link</b></td>
        <td>string</td>
        <td>
          link is the link on which the network applies<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindex">routes</a></b></td>
        <td>[]object</td>
        <td>
          routes contains a list of IPv4 routes<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index].routes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4dhcpindex)</sup></sup>



NetworkDataRoutev4 represents an ipv4 route object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexgateway">gateway</a></b></td>
        <td>object</td>
        <td>
          gateway is the IPv4 address of the gateway<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>network</b></td>
        <td>string</td>
        <td>
          network is the IPv4 network address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          prefix is the mask of the network as integer (max 32)<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 32<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexservices">services</a></b></td>
        <td>object</td>
        <td>
          services is a list of IPv4 services<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index].routes[index].gateway
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindex)</sup></sup>



gateway is the IPv4 address of the gateway

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fromIPPool</b></td>
        <td>string</td>
        <td>
          fromIPPool is the name of the IPPool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexgatewayfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexgatewayfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string is the gateway given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index].routes[index].gateway.fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexgateway)</sup></sup>



fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index].routes[index].gateway.fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindexgateway)</sup></sup>



fromPoolRef is a reference to a IP pool to fetch the gateway from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv4DHCP[index].routes[index].services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv4dhcpindexroutesindex)</sup></sup>



services is a list of IPv4 services

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of IPv4 DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworks)</sup></sup>



NetworkDataIPv6 represents an ipv6 static network object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the network ID (name)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>link</b></td>
        <td>string</td>
        <td>
          link is the link on which the network applies<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool reference via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, ipAddressFromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to allocate an address from.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ipAddressFromIPPool</b></td>
        <td>string</td>
        <td>
          ipAddressFromIPPool contains the name of the IPPool to use to get an ip address<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindex">routes</a></b></td>
        <td>[]object</td>
        <td>
          routes contains a list of IPv6 routes<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6index)</sup></sup>



fromPoolAnnotation allows specifying the pool reference via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, ipAddressFromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6index)</sup></sup>



fromPoolRef is a reference to a IP pool to allocate an address from.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].routes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6index)</sup></sup>



NetworkDataRoutev6 represents an ipv6 route object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexgateway">gateway</a></b></td>
        <td>object</td>
        <td>
          gateway is the IPv6 address of the gateway<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>network</b></td>
        <td>string</td>
        <td>
          network is the IPv6 network address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexservices">services</a></b></td>
        <td>object</td>
        <td>
          services is a list of IPv6 services<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].routes[index].gateway
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindex)</sup></sup>



gateway is the IPv6 address of the gateway

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fromIPPool</b></td>
        <td>string</td>
        <td>
          fromIPPool is the name of the IPPool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexgatewayfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexgatewayfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string is the gateway given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].routes[index].gateway.fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexgateway)</sup></sup>



fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].routes[index].gateway.fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindexgateway)</sup></sup>



fromPoolRef is a reference to a IP pool to fetch the gateway from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6[index].routes[index].services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6indexroutesindex)</sup></sup>



services is a list of IPv6 services

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of IPv6 DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworks)</sup></sup>



NetworkDataIPv6DHCP represents an ipv6 DHCP network object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the network ID (name)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>link</b></td>
        <td>string</td>
        <td>
          link is the link on which the network applies<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindex">routes</a></b></td>
        <td>[]object</td>
        <td>
          routes contains a list of IPv6 routes<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index].routes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6dhcpindex)</sup></sup>



NetworkDataRoutev6 represents an ipv6 route object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexgateway">gateway</a></b></td>
        <td>object</td>
        <td>
          gateway is the IPv6 address of the gateway<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>network</b></td>
        <td>string</td>
        <td>
          network is the IPv6 network address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexservices">services</a></b></td>
        <td>object</td>
        <td>
          services is a list of IPv6 services<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index].routes[index].gateway
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindex)</sup></sup>



gateway is the IPv6 address of the gateway

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fromIPPool</b></td>
        <td>string</td>
        <td>
          fromIPPool is the name of the IPPool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexgatewayfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexgatewayfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string is the gateway given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index].routes[index].gateway.fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexgateway)</sup></sup>



fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index].routes[index].gateway.fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindexgateway)</sup></sup>



fromPoolRef is a reference to a IP pool to fetch the gateway from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6DHCP[index].routes[index].services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6dhcpindexroutesindex)</sup></sup>



services is a list of IPv6 services

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of IPv6 DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworks)</sup></sup>



NetworkDataIPv6DHCP represents an ipv6 DHCP network object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>id</b></td>
        <td>string</td>
        <td>
          id is the network ID (name)<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>link</b></td>
        <td>string</td>
        <td>
          link is the link on which the network applies<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindex">routes</a></b></td>
        <td>[]object</td>
        <td>
          routes contains a list of IPv6 routes<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index].routes[index]
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6slaacindex)</sup></sup>



NetworkDataRoutev6 represents an ipv6 route object.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexgateway">gateway</a></b></td>
        <td>object</td>
        <td>
          gateway is the IPv6 address of the gateway<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>network</b></td>
        <td>string</td>
        <td>
          network is the IPv6 network address<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>prefix</b></td>
        <td>integer</td>
        <td>
          prefix is the mask of the network as integer (max 128)<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
            <i>Maximum</i>: 128<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexservices">services</a></b></td>
        <td>object</td>
        <td>
          services is a list of IPv6 services<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index].routes[index].gateway
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindex)</sup></sup>



gateway is the IPv6 address of the gateway

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>fromIPPool</b></td>
        <td>string</td>
        <td>
          fromIPPool is the name of the IPPool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexgatewayfrompoolannotation">fromPoolAnnotation</a></b></td>
        <td>object</td>
        <td>
          fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexgatewayfrompoolref">fromPoolRef</a></b></td>
        <td>object</td>
        <td>
          fromPoolRef is a reference to a IP pool to fetch the gateway from<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>string</b></td>
        <td>string</td>
        <td>
          string is the gateway given as a string<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index].routes[index].gateway.fromPoolAnnotation
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexgateway)</sup></sup>



fromPoolAnnotation allows specifying the pool name via an annotation on
a Machine, Metal3Machine, or BareMetalHost object.
When set, fromIPPool and fromPoolRef are ignored.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>annotation</b></td>
        <td>string</td>
        <td>
          annotation is the key of the annotation that contains the pool name.
The annotation value should be a string containing the pool name.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>object</b></td>
        <td>enum</td>
        <td>
          object is the type of the object from which we retrieve the annotation<br/>
          <br/>
            <i>Enum</i>: machine, metal3machine, baremetalhost<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index].routes[index].gateway.fromPoolRef
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindexgateway)</sup></sup>



fromPoolRef is a reference to a IP pool to fetch the gateway from

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the IPPool.
name must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character.<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>apiGroup</b></td>
        <td>string</td>
        <td>
          apiGroup of the IPPool.
apiGroup must be fully qualified domain name.<br/>
          <br/>
            <i>Default</i>: ipam.metal3.io<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>kind</b></td>
        <td>string</td>
        <td>
          kind of the IPPool.
kind must consist of alphanumeric characters or '-', start with an alphabetic character, and end with an alphanumeric character.<br/>
          <br/>
            <i>Default</i>: IPPool<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.networks.ipv6SLAAC[index].routes[index].services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdatanetworksipv6slaacindexroutesindex)</sup></sup>



services is a list of IPv6 services

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of IPv6 DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.spec.networkData.services
<sup><sup>[↩ Parent](#metal3datatemplatespecnetworkdata)</sup></sup>



services  is a structure containing lists of different types objects

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>dns</b></td>
        <td>[]string</td>
        <td>
          dns is a list of DNS services<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>dnsFromIPPool</b></td>
        <td>string</td>
        <td>
          dnsFromIPPool is the name of the IPPool from which to get the DNS servers<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.status
<sup><sup>[↩ Parent](#metal3datatemplate)</sup></sup>



status defines the observed state of Metal3DataTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3datatemplatestatusindexesindex">indexes</a></b></td>
        <td>[]object</td>
        <td>
          indexes contains the list of Metal3Machines and their assigned indexes<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>lastUpdated</b></td>
        <td>string</td>
        <td>
          lastUpdated identifies when this status was last observed.<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataTemplate.status.indexes[index]
<sup><sup>[↩ Parent](#metal3datatemplatestatus)</sup></sup>



IndexEntry represents the index assignment for a Metal3Machine.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>index</b></td>
        <td>integer</td>
        <td>
          index is the index value assigned to the Machine<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>true</td>
      </tr><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is the name of the Metal3Machine<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>

## Metal3DataClaim
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3DataClaim is the Schema for the metal3datas API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3DataClaim</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3dataclaimspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3DataClaim.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3dataclaimstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3DataClaim.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataClaim.spec
<sup><sup>[↩ Parent](#metal3dataclaim)</sup></sup>



spec defines the desired state of Metal3DataClaim.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3dataclaimspectemplate">template</a></b></td>
        <td>object</td>
        <td>
          template is the Metal3DataTemplate this was generated for.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataClaim.spec.template
<sup><sup>[↩ Parent](#metal3dataclaimspec)</sup></sup>



template is the Metal3DataTemplate this was generated for.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataClaim.status
<sup><sup>[↩ Parent](#metal3dataclaim)</sup></sup>



status defines the observed state of Metal3DataClaim.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>errorMessage</b></td>
        <td>string</td>
        <td>
          errorMessage contains the error message<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3dataclaimstatusrendereddata">renderedData</a></b></td>
        <td>object</td>
        <td>
          renderedData references the Metal3Data when ready<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3DataClaim.status.renderedData
<sup><sup>[↩ Parent](#metal3dataclaimstatus)</sup></sup>



renderedData references the Metal3Data when ready

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3Data
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3Data is the Schema for the metal3datas API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3Data</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3dataspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3Data.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3datastatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3Data.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.spec
<sup><sup>[↩ Parent](#metal3data)</sup></sup>



spec defines the desired state of Metal3Data.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3dataspecclaim">claim</a></b></td>
        <td>object</td>
        <td>
          claim points to the Metal3DataClaim the Metal3Data was created for.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>index</b></td>
        <td>integer</td>
        <td>
          index stores the index value of this instance in the Metal3DataTemplate.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 0<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3dataspecmetadata">metaData</a></b></td>
        <td>object</td>
        <td>
          metaData points to the rendered MetaData secret.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3dataspecnetworkdata">networkData</a></b></td>
        <td>object</td>
        <td>
          networkData points to the rendered NetworkData secret.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3dataspectemplate">template</a></b></td>
        <td>object</td>
        <td>
          template is the Metal3DataTemplate this was generated from.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.spec.claim
<sup><sup>[↩ Parent](#metal3dataspec)</sup></sup>



claim points to the Metal3DataClaim the Metal3Data was created for.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.spec.metaData
<sup><sup>[↩ Parent](#metal3dataspec)</sup></sup>



metaData points to the rendered MetaData secret.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.spec.networkData
<sup><sup>[↩ Parent](#metal3dataspec)</sup></sup>



networkData points to the rendered NetworkData secret.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name is unique within a namespace to reference a secret resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace defines the space within which the secret name must be unique.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.spec.template
<sup><sup>[↩ Parent](#metal3dataspec)</sup></sup>



template is the Metal3DataTemplate this was generated from.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>name</b></td>
        <td>string</td>
        <td>
          name of the resource.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>namespace</b></td>
        <td>string</td>
        <td>
          namespace of the resource.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Data.status
<sup><sup>[↩ Parent](#metal3data)</sup></sup>



status defines the observed state of Metal3Data.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>errorMessage</b></td>
        <td>string</td>
        <td>
          errorMessage contains the error message<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>ready</b></td>
        <td>boolean</td>
        <td>
          ready is a flag set to True if the secrets were rendered properly<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3Remediation
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3Remediation is the Schema for the metal3remediations API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3Remediation</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3remediationspec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3Remediation.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3remediationstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3Remediation.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Remediation.spec
<sup><sup>[↩ Parent](#metal3remediation)</sup></sup>



spec defines the desired state of Metal3Remediation.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3remediationspecstrategy">strategy</a></b></td>
        <td>object</td>
        <td>
          strategy field defines remediation strategy.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Remediation.spec.strategy
<sup><sup>[↩ Parent](#metal3remediationspec)</sup></sup>



strategy field defines remediation strategy.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>retryLimit</b></td>
        <td>integer</td>
        <td>
          retryLimit sets maximum number of remediation retries.
retryLimit must be a positive integer. The default is 1.
Setting a very high value is discouraged: the controller waits at
least timeoutSeconds (minimum 100s) between attempts, so a large
retryLimit keeps the host in a reboot loop for a very long time
before remediation is allowed to fail.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>timeoutSeconds</b></td>
        <td>integer</td>
        <td>
          timeoutSeconds defines the timeout between remediation retries.
The minimum allowed value is 100 seconds. The default is 600 seconds.
Setting a very high value is discouraged: the controller waits this
long between attempts, so a large timeout delays every retry and can
effectively stall remediation for a very long time.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 600<br/>
            <i>Minimum</i>: 100<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          type of remediation.<br/>
          <br/>
            <i>Enum</i>: Reboot<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3Remediation.status
<sup><sup>[↩ Parent](#metal3remediation)</sup></sup>



status defines the observed state of Metal3Remediation.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastRemediated</b></td>
        <td>string</td>
        <td>
          lastRemediated identifies when the host was last remediated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>phase</b></td>
        <td>enum</td>
        <td>
          phase represents the current phase of machine remediation.
E.g. Pending, Running, Done etc.<br/>
          <br/>
            <i>Enum</i>: Running, Waiting, Deleting machine, Failed<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>retryCount</b></td>
        <td>integer</td>
        <td>
          retryCount can be used as a counter during the remediation.
Field can hold number of reboots etc.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>

## Metal3RemediationTemplate
<sup><sup>[↩ Parent](#infrastructureclusterx-k8siov1beta2 )</sup></sup>






Metal3RemediationTemplate is the Schema for the metal3remediationtemplates API.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>infrastructure.cluster.x-k8s.io/v1beta2</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>Metal3RemediationTemplate</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b><a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.37/#objectmeta-v1-meta">metadata</a></b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b><a href="#metal3remediationtemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec defines the desired state of Metal3RemediationTemplate.<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b><a href="#metal3remediationtemplatestatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3RemediationTemplate.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.spec
<sup><sup>[↩ Parent](#metal3remediationtemplate)</sup></sup>



spec defines the desired state of Metal3RemediationTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3remediationtemplatespectemplate">template</a></b></td>
        <td>object</td>
        <td>
          template describes the data needed to create a Metal3Remediation from a template<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.spec.template
<sup><sup>[↩ Parent](#metal3remediationtemplatespec)</sup></sup>



template describes the data needed to create a Metal3Remediation from a template

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3remediationtemplatespectemplatespec">spec</a></b></td>
        <td>object</td>
        <td>
          spec is the specification of the desired behavior of the Metal3Remediation.<br/>
        </td>
        <td>true</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.spec.template.spec
<sup><sup>[↩ Parent](#metal3remediationtemplatespectemplate)</sup></sup>



spec is the specification of the desired behavior of the Metal3Remediation.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3remediationtemplatespectemplatespecstrategy">strategy</a></b></td>
        <td>object</td>
        <td>
          strategy field defines remediation strategy.<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.spec.template.spec.strategy
<sup><sup>[↩ Parent](#metal3remediationtemplatespectemplatespec)</sup></sup>



strategy field defines remediation strategy.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>retryLimit</b></td>
        <td>integer</td>
        <td>
          retryLimit sets maximum number of remediation retries.
retryLimit must be a positive integer. The default is 1.
Setting a very high value is discouraged: the controller waits at
least timeoutSeconds (minimum 100s) between attempts, so a large
retryLimit keeps the host in a reboot loop for a very long time
before remediation is allowed to fail.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 1<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>timeoutSeconds</b></td>
        <td>integer</td>
        <td>
          timeoutSeconds defines the timeout between remediation retries.
The minimum allowed value is 100 seconds. The default is 600 seconds.
Setting a very high value is discouraged: the controller waits this
long between attempts, so a large timeout delays every retry and can
effectively stall remediation for a very long time.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Default</i>: 600<br/>
            <i>Minimum</i>: 100<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>type</b></td>
        <td>enum</td>
        <td>
          type of remediation.<br/>
          <br/>
            <i>Enum</i>: Reboot<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.status
<sup><sup>[↩ Parent](#metal3remediationtemplate)</sup></sup>



status defines the observed state of Metal3RemediationTemplate.

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b><a href="#metal3remediationtemplatestatusstatus">status</a></b></td>
        <td>object</td>
        <td>
          status defines the observed state of Metal3Remediation<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>


### Metal3RemediationTemplate.status.status
<sup><sup>[↩ Parent](#metal3remediationtemplatestatus)</sup></sup>



status defines the observed state of Metal3Remediation

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>lastRemediated</b></td>
        <td>string</td>
        <td>
          lastRemediated identifies when the host was last remediated<br/>
          <br/>
            <i>Format</i>: date-time<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>phase</b></td>
        <td>enum</td>
        <td>
          phase represents the current phase of machine remediation.
E.g. Pending, Running, Done etc.<br/>
          <br/>
            <i>Enum</i>: Running, Waiting, Deleting machine, Failed<br/>
        </td>
        <td>false</td>
      </tr><tr>
        <td><b>retryCount</b></td>
        <td>integer</td>
        <td>
          retryCount can be used as a counter during the remediation.
Field can hold number of reboots etc.<br/>
          <br/>
            <i>Format</i>: int32<br/>
            <i>Minimum</i>: 1<br/>
        </td>
        <td>false</td>
      </tr></tbody>
</table>
