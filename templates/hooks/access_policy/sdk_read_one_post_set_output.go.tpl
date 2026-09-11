	if resp.AccessPolicyDetail != nil && resp.AccessPolicyDetail.Policy != nil {
		policy, err := resp.AccessPolicyDetail.Policy.MarshalSmithyDocument()
		if err != nil {
			return &resource{ko}, err
		}
		ko.Spec.Policy = aws.String(string(policy))
	}
