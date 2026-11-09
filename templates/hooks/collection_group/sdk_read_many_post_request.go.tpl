	if err == nil {
    if itemErr := collectionGroupReadError(resp); itemErr != nil {
        // Per-item codes are distinct from operation-level exceptions. Only
        // the documented NOT_FOUND code may become a missing-resource result.
        rm.metrics.RecordAPICall("READ_MANY", "BatchGetCollectionGroup", itemErr)
        return nil, itemErr
    }
	}
