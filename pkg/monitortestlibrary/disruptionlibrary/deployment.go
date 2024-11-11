package disruptionlibrary

import (
	appsv1 "k8s.io/api/apps/v1"
)

func UpdateDeploymentENVs(deployment *appsv1.Deployment, deploymentID, serviceClusterIP string) *appsv1.Deployment {
	for i, env := range deployment.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "DEPLOYMENT_ID" {
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = deploymentID
		} else if env.Name == "SERVICE_CLUSTER_IP" && len(serviceClusterIP) > 0 {
			deployment.Spec.Template.Spec.Containers[0].Env[i].Value = serviceClusterIP
		}
	}

	return deployment
}
