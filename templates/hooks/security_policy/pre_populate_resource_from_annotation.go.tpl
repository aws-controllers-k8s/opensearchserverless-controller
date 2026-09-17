	// Backward compatibility: earlier releases of this controller generated the
	// adoption annotation key for the Type field as "type_" (the reserved-word
	// escaped Go name) rather than the CRD field name "type". Alias the legacy
	// key to the current one so existing adoption annotations keep working. If
	// both keys are supplied the annotation is ambiguous, so reject it.
	if _, hasNew := fields["type"]; hasNew {
		if _, hasLegacy := fields["type_"]; hasLegacy {
			return ackerrors.NewTerminalError(fmt.Errorf("adoption annotation must not set both \"type\" and the deprecated \"type_\""))
		}
	} else if legacy, hasLegacy := fields["type_"]; hasLegacy {
		fields["type"] = legacy
	}
