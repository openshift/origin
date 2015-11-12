This example depends on the existence of a "php" imagestream in the "openshift" namespace.  If you do not have one defined, you can create it by using one of the imagestream definition files found here:
https://github.com/openshift/origin/tree/master/examples/image-streams

	oc create -f image-streams-centos7.json -n openshift

(you will need to be a cluster admin to create imagestreams in the openshift namespace)

To use this example, instantiate it with

	oc new-app -f wordpress-mysql.json

Take note of the `DATABASE_PASSWORD` that is displayed.

Wait for the build of the new wordpress image to complete.  You can monitor the build by running

	oc get builds --watch

Once the wordpress build completes, determine the wordpress and
mysql service IPs by running:

	oc get svc

Navigate to `<wordpress service ip>:8080` in your browser.  You will
be prompted to setup wordpress.

For the database hostname, provide the mysql service ip.

For the database username, enter "wordpress"

For the database password, provide the password generated when you
instantiated the template.

You should not need to change any other values.


Note: this template uses an EmptyDir volume type for database storage.  This type of storage is not persisted.  If you want to ensure the database content is not lost, modify the template to user a persistent volume claim type instead.





