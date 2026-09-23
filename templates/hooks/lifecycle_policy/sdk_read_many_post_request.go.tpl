	// BatchGetLifecyclePolicy reports per-identifier failures in the response
	// body (LifecyclePolicyErrorDetails) rather than as a top-level API
	// exception, so the generic 404-exception check below never fires. A
	// missing policy is signaled by ErrorCode == "NOT_FOUND"; any other error
	// code is a genuine read failure and must be surfaced as an error rather
	// than masked as a not-found (which would trigger a spurious re-Create).
	// Exactly one identifier is requested per reconcile, so there is at most
	// one error detail and returning on the first is sufficient.
	if err == nil && resp != nil {
		for _, errDetail := range resp.LifecyclePolicyErrorDetails {
			if errDetail.ErrorCode == nil {
				continue
			}
			if *errDetail.ErrorCode == "NOT_FOUND" {
				return nil, ackerr.NotFound
			}
			errMsg := ""
			if errDetail.ErrorMessage != nil {
				errMsg = *errDetail.ErrorMessage
			}
			return nil, fmt.Errorf(
				"BatchGetLifecyclePolicy failed for policy: %s: %s",
				*errDetail.ErrorCode, errMsg,
			)
		}
		if len(resp.LifecyclePolicyDetails) == 0 {
			return nil, ackerr.NotFound
		}
	}
