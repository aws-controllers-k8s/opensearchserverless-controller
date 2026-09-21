	// BatchGetLifecyclePolicy signals a missing policy in the response body
	// (LifecyclePolicyErrorDetails[].ErrorCode == "NOT_FOUND") rather than via
	// a top-level API exception, so the generic 404-exception check below never
	// fires. Translate the in-body NOT_FOUND into ackerr.NotFound here.
	if err == nil && resp != nil {
		for _, errDetail := range resp.LifecyclePolicyErrorDetails {
			if errDetail.ErrorCode != nil && *errDetail.ErrorCode == "NOT_FOUND" {
				return nil, ackerr.NotFound
			}
		}
		if len(resp.LifecyclePolicyDetails) == 0 {
			return nil, ackerr.NotFound
		}
	}
