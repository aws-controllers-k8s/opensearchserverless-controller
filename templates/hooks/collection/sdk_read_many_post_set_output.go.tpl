	ko.Spec.Tags, err = getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN), rm.sdkapi, rm.metrics)
	if err != nil {
		return &resource{ko}, err
	}

	if !collectionIsActive(&resource{ko}) {
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, aws.String("collection is not active"), nil)
	}
