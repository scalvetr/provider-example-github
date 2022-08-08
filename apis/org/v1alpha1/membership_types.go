/*
Copyright 2020 The Crossplane Authors.

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

package v1alpha1

import (
	"reflect"

	xpv1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// MembershipParameters are the configurable fields of a Membership.
type MembershipParameters struct {
	// The name of the organization to which the user should be added.
	Org string `json:"org"`

	// The name of the used to be granted membership.
	User string `json:"user"`

	// Team is the name of the team to which the user should be added.
	Team *string `json:"team,omitempty"`

	// TeamRef referes to a Team resource.
	TeamRef *xpv1.Reference `json:"teamRef,omitempty"`

	// TeamSelector selects one Team resource.
	TeamSelector *xpv1.Selector `json:"teamSelector,omitempty"`
}

// MembershipObservation are the observable fields of a Membership.
type MembershipObservation struct {
	State string `json:"state,omitempty"`
}

// A MembershipSpec defines the desired state of a Membership.
type MembershipSpec struct {
	xpv1.ResourceSpec `json:",inline"`
	ForProvider       MembershipParameters `json:"forProvider"`
}

// A MembershipStatus represents the observed state of a Membership.
type MembershipStatus struct {
	xpv1.ResourceStatus `json:",inline"`
	AtProvider          MembershipObservation `json:"atProvider,omitempty"`
}

// +kubebuilder:object:root=true

// A Membership is an example API type
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="SYNCED",type="string",JSONPath=".status.conditions[?(@.type=='Synced')].status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:resource:scope=Cluster
type Membership struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MembershipSpec   `json:"spec"`
	Status MembershipStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MembershipList contains a list of Membership
type MembershipList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Membership `json:"items"`
}

// Membership type metadata.
var (
	MembershipKind             = reflect.TypeOf(Membership{}).Name()
	MembershipGroupKind        = schema.GroupKind{Group: Group, Kind: MembershipKind}.String()
	MembershipKindAPIVersion   = MembershipKind + "." + SchemeGroupVersion.String()
	MembershipGroupVersionKind = SchemeGroupVersion.WithKind(MembershipKind)
)

func init() {
	SchemeBuilder.Register(&Membership{}, &MembershipList{})
}
