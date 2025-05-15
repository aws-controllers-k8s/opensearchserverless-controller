	r.ko.Spec.Tags, err = getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN), rm.sdkapi, rm.metrics)
	if err != nil {
		return &resource{ko}, err
	}