/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

func (*Metal3Cluster) Hub()                 {}
func (*Metal3ClusterList) Hub()             {}
func (*Metal3ClusterTemplate) Hub()         {}
func (*Metal3Machine) Hub()                 {}
func (*Metal3MachineList) Hub()             {}
func (*Metal3MachineTemplate) Hub()         {}
func (*Metal3MachineTemplateList) Hub()     {}
func (*Metal3DataTemplate) Hub()            {}
func (*Metal3DataTemplateList) Hub()        {}
func (*Metal3Data) Hub()                    {}
func (*Metal3DataList) Hub()                {}
func (*Metal3DataClaim) Hub()               {}
func (*Metal3DataClaimList) Hub()           {}
func (*Metal3Remediation) Hub()             {}
func (*Metal3RemediationList) Hub()         {}
func (*Metal3RemediationTemplate) Hub()     {}
func (*Metal3RemediationTemplateList) Hub() {}
