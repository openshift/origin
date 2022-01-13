package allowedalerts

func AllAlertTests() []AlertTest {
	return []AlertTest{
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").pending().neverFail(),
		newAlert("kube-apiserver", "KubeAPIErrorBudgetBurn").firing(),
	}
}
