# Pruning images using CronJob

This example shows an image pruning happening in an automated fashion using the Kubernetes [CronJobs](https://docs.openshift.org/latest/dev_guide/cron_jobs.html) that
are available in OpenShift Origin starting from version 3.5.
In this example, we will create a CronJob that will run image pruning every 1 hour.

## Requirements

In order to execute the pruning commands successfully, it is necessary to configure the
authorization in a way that allows the `default` service account to perform the pruning
against entire cluster (assuming you create the CronJob in the `default` project):

1. `oc adm policy add-cluster-role-to-user system:image-pruner system:serviceaccount:default:default --config=admin.kubeconfig`

    This command will grant the "image-pruner" role to service account in the `default`
    namespace. That will allow the service account to list all images in the cluster and
    perform the image pruning.

## Creating the CronJob

2. `oc create -f examples/pruner/job.yaml -n default --config=admin.kubeconfig`

    This command creates the CronJob resource that runs the pruning job every 1 hour.

Make sure, that you check the `oc adm prune --help` command and optionally tweak the
CronJob arguments by specifying how much tag revisions you want to preserve on a single
tag or other options that might suit your environment.

## Cleaning up old jobs

To cleanup finished jobs, you can run this command:

`oc delete jobs -l job=prune-images`

Note that starting from Origin version 3.6, you will be able to specify `successfulJobsHistoryLimit` and `failedJobsHistoryLimit`
options for the CronJob, so the cleanup command above won't be needed.
