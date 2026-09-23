	// The Policy field is a Smithy document (document.Interface). The generic
	// output-setter cannot marshal it, so populate Spec.Policy from the batch
	// detail here. NOT_FOUND / empty-result cases already returned above, so a
	// detail is present.
	if len(resp.LifecyclePolicyDetails) > 0 {
		detail := resp.LifecyclePolicyDetails[0]
		if detail.Policy != nil {
			policyBytes, err := detail.Policy.MarshalSmithyDocument()
			if err != nil {
				return &resource{ko}, err
			}
			ko.Spec.Policy = aws.String(string(policyBytes))
		}
	}
