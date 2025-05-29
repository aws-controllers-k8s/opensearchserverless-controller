package security_policy

import (
	"encoding/json"
	"reflect"

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
		comparePolicy(delta, a, b)
	}
}

func comparePolicy(delta *ackcompare.Delta, a *resource, b *resource) {
	if b.ko.Spec.Type == nil {
		return
	}
	switch *b.ko.Spec.Type {
	case string(svcsdktypes.SecurityPolicyTypeEncryption):
		var desiredPolicy EncryptionPolicy
		var latestPolicy EncryptionPolicy
		err := json.Unmarshal([]byte(*a.ko.Spec.Policy), &desiredPolicy)
		if err != nil {
			delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
			return
		}
		err = json.Unmarshal([]byte(*b.ko.Spec.Policy), &latestPolicy)
		if err != nil {
			delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
			return
		}
		if !reflect.DeepEqual(desiredPolicy, latestPolicy) {
			delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		}

	case string(svcsdktypes.SecurityPolicyTypeNetwork):
		var desired []NetworkPolicy
		var latest []NetworkPolicy
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
		}
		if !reflect.DeepEqual(desired, latest) {
			delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
		}
	}

}
