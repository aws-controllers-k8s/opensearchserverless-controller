	if err = rm.setTags(ctx, ko); err != nil {
		return &resource{ko}, err
	}

	if err = rm.setMemberCollections(ctx, ko); err != nil {
		return &resource{ko}, err
	}
