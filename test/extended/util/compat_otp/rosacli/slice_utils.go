package rosacli

func RemoveFromStringSlice(slice []string, value string) []string {
	var newSlice []string
	for _, v := range slice {
		if v != value {
			newSlice = append(newSlice, v)
		}
	}
	return newSlice
}

func SliceContains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

func AppendToStringSliceIfNotExist(slice []string, value string) []string {
	if !SliceContains(slice, value) {
		slice = append(slice, value)
	}
	return slice
}
