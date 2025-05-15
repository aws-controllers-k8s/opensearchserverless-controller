	desired.SetStatus(latest)
	if delta.DifferentAt("Spec.Tags") {
		arn := string(*latest.ko.Status.ACKResourceMetadata.ARN)
		err = syncTags(
			ctx, 
			desired.ko.Spec.Tags, latest.ko.Spec.Tags, 
			&arn, convertToOrderedACKTags, rm.sdkapi, rm.metrics,
		)
		if err != nil {
			return desired, err
		}
	}
	if !delta.DifferentExcept("Spec.Tags") {
		return desired, nil
	}
