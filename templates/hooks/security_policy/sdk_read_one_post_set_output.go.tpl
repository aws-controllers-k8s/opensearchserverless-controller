	policy, err := resp.SecurityPolicyDetail.Policy.MarshalSmithyDocument()
	if err != nil {
		return &resource{ko}, err
	}
	ko.Spec.Policy = aws.String(string(policy))
