	// Backward compatibility: earlier releases of this controller used the
	// adoption AdditionalKeys entry "type_" (the reserved-word escaped Go name)
	// rather than the CRD field name "type". Alias the legacy key to the current
	// one so existing adoption annotations keep working. If both keys are
	// supplied the annotation is ambiguous, so reject it.
	if identifier.AdditionalKeys != nil {
		if _, hasNew := identifier.AdditionalKeys["type"]; hasNew {
			if _, hasLegacy := identifier.AdditionalKeys["type_"]; hasLegacy {
				return ackerrors.NewTerminalError(fmt.Errorf("adoption annotation must not set both \"type\" and the deprecated \"type_\""))
			}
		} else if legacy, hasLegacy := identifier.AdditionalKeys["type_"]; hasLegacy {
			identifier.AdditionalKeys["type"] = legacy
		}
	}
