package security_policy

import (
	"encoding/json"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"
)

// NetworkPolicy is a Policy of type Network
type NetworkPolicy struct {
	Rules           []NetworkRule `json:"Rules"`
	Description     string        `json:"Description"`
	AllowFromPublic bool          `json:"AllowFromPublic"`
}

// NetworkRule is a Rule for Network type
type NetworkRule struct {
	ResourceType    string   `json:"ResourceType"`
	Resource        []string `json:"Resource"`
	AllowFromPublic bool     `json:"AllowFromPublic"`
}

// EncryptionPolicy is a Policy of type Encryption
type EncryptionPolicy struct {
	AWSOwnedKey bool             `json:"AWSOwnedKey"`
	Rules       []EncryptionRule `json:"Rules"`
	KmsARN      string           `json:"KmsARN"`
}

// EncryptionRule is a Rule for Encruption type
type EncryptionRule struct {
	ResourceType string   `json:"ResourceType"`
	Resource     []string `json:"Resource"`
}

func customPreCompare(delta *ackcompare.Delta, a *resource, b *resource) {
	if ackcompare.HasNilDifference(a.ko.Spec.Policy, b.ko.Spec.Policy) {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
	} else if a.ko.Spec.Policy != nil && b.ko.Spec.Policy != nil {
		if b.ko.Spec.Type == nil {
			return
		}
		// SecurityPolicy can either be of type Encryption or Network. We will need to convert
		// the Policy to its appropriate struct so we can ensure we don't have false diffs
		// (eg. extra whitespace diffs). 
		if *b.ko.Spec.Type == string(svcsdktypes.SecurityPolicyTypeEncryption) {
			compareEncryptionPolicy(delta, a, b)
		} else {
			compareNetworkPolicy(delta, a, b)
		}
	}
}

func compareEncryptionPolicy(delta *ackcompare.Delta, a *resource, b *resource) {
	var desired EncryptionPolicy
	var latest EncryptionPolicy

	// Since Delta does not return an error, we will rely on the sdkUpdate call
	// to catch any errors that ocurr during marshalling/unmarshalling Policies
	err := json.Unmarshal([]byte(*a.ko.Spec.Policy), &desired)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	err = json.Unmarshal([]byte(*b.ko.Spec.Policy), &latest)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}

	desiredPolicyString, err := json.Marshal(desired)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	// TODO: Marshalling and unmarshalling latest may be useless since
	// it is Marshalled during sdkFind. I propose we remove it?
	latestPolicyString, err := json.Marshal(latest)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	if string(desiredPolicyString) != string(latestPolicyString) {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
	}
}

func compareNetworkPolicy(delta *ackcompare.Delta, a *resource, b *resource) {
	var desired []NetworkPolicy
	var latest []NetworkPolicy

	// Since Delta does not return an error, we will rely on the sdkUpdate call
	// to catch any errors that ocurr during marshalling/unmarshalling Policies
	err := json.Unmarshal([]byte(*a.ko.Spec.Policy), &desired)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	err = json.Unmarshal([]byte(*b.ko.Spec.Policy), &latest)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}

	if len(desired) != len(latest) {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	desiredPolicyString, err := json.Marshal(desired)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}
	// TODO: Marshalling and unmarshalling latest may be useless since
	// it is Marshalled during sdkFind. I propose we remove it?
	latestPolicyString, err := json.Marshal(latest)
	if err != nil {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		return
	}

	if string(desiredPolicyString) != string(latestPolicyString) {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
	}
}
