package failure_detector

// SimpleWeightedEndpointStatusEvaluator an external policy evaluator that sets the status and weight of the given endpoint based on the collected samples.
// It returns true only if status or wight have changed otherwise false
//
// WeightedEndpointStatus.Status:
// will be set to EndpointStatusReasonTooManyErrors only when it sees 10 errors
// otherwise it will be set to an empty string
//
// WeightedEndpointStatus.Weight:
// 10 consecutive will decrease the weight by 0.01 for example:
//  - the value of 1 means no errors
//  - the value of 0 means it observed 100 errors
//  - the value of 0.7 means it observed 30 errors
func SimpleWeightedEndpointStatusEvaluator(endpoint *WeightedEndpointStatus) bool {
	errThreshold := 10
	errCount := 0

	for _, sample := range endpoint.data {
		if sample != nil && sample.err != nil {
			errCount++
		}
		if sample != nil && sample.err == nil {
			errCount--
		}
	}

	abs := func(x int) int {
		if x < 0 {
			return x * -1
		}
		return x
	}

	if abs(errCount) != errThreshold {
		return false
	}

	prevErrCount := weightToErrorCount(endpoint.weight)
	totalErrCount := prevErrCount + errCount
	if totalErrCount < 0 || totalErrCount > errThreshold*10 {
		return false
	}

	hasChanged := false
	newWeight := 1 - 0.01*float32(totalErrCount)
	if prevErrCount != totalErrCount {
		endpoint.weight = newWeight
		hasChanged = true
	}

	// reset the buffer
	if hasChanged {
		endpoint.data = make([]*Sample, endpoint.size, endpoint.size)
		endpoint.position = 0
	}

	if endpoint.weight <= 0.0 && endpoint.status != EndpointStatusReasonTooManyErrors {
		endpoint.status = EndpointStatusReasonTooManyErrors
		hasChanged = true
	} else if endpoint.status != "" {
		endpoint.status = ""
		hasChanged = true
	}

	return hasChanged
}

func weightToErrorCount(weight float32) int {
	return 100 - int(weight*100)
}
