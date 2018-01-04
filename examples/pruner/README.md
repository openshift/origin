# Pruning images using CronJob

This example shows an image pruning happening in an automated fashion using the
Kubernetes [CronJobs](https://docs.openshift.org/latest/dev_guide/cron_jobs.html).
In this example, we will create a CronJob that will run image pruning every 1 hour.

## Requirements

In order to execute the pruning commands successfully, it is necessary to create a
dedicated service account `image-pruner` with necessary privileges to perform pruning
against the entire cluster.  Make sure you run below commands with a user who has
the power to assign cluster roles.  Also double check the namespace you are invoking
them in, if it is the one you desire.

1. `oc create serviceaccount image-pruner`

    This command creates an `image-pruner` [service account](https://docs.openshift.org/latest/admin_guide/service_accounts.html).

2. `oc adm policy add-cluster-role-to-user system:image-pruner image-pruner`

    This command grants the `image-pruner` [role](https://docs.openshift.org/latest/admin_guide/manage_rbac.html) to that service account.

## Creating the CronJob

2. `oc create -f examples/pruner/cronjob.yaml -n default --config=admin.kubeconfig`

    This command creates the CronJob resource that runs the pruning job every 1 hour.

Make sure, that you check the `oc adm prune --help` command and optionally tweak the
CronJob arguments by specifying how much tag revisions you want to preserve on a single
tag or other options that might suit your environment.  Full details about pruning images
can be found in the [official documentation](https://docs.openshift.org/latest/admin_guide/pruning_resources.html#pruning-images).
