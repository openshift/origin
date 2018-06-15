# Using S2I images

S2I builder images normally include [`assemble` and `run` scripts](https://docs.openshift.org/latest/creating_images/s2i.html#s2i-scripts), but the default behavior of those scripts may not be suitable for all users. This topic covers a few approaches for customizing the behavior of an S2I builder that includes default scripts.

## Invoking scripts embedded in an image

Typically, builder images provide their own version of the [S2I scripts](builder_image.md#s2i-scripts) that cover the most common use-cases. If these scripts do not fulfill your needs, S2I provides a way of overriding them by adding custom ones in the `.s2i/bin` directory. However, by doing this you are [completely replacing the standard scripts](https://docs.openshift.org/latest/creating_images/s2i.html#s2i-scripts). In some cases this is acceptable, but in other scenarios you may prefer to execute a few commands before (or after) the scripts while retaining the logic of the script provided in the image. In this case, it is possible to create a wrapper script that executes custom logic and delegates further work to the default script in the image.

To determine the location of the scripts inside of the builder image, look at the value of `io.openshift.s2i.scripts-url` label. Use `docker inspect`:

```console
$ docker inspect --format='{{ index .Config.Labels "io.openshift.s2i.scripts-url" }}' openshift/wildfly-100-centos7
image:///usr/libexec/s2i
```

You inspected the `openshift/wildfly-100-centos7` builder image and found out that the scripts are in the `/usr/libexec/s2i` directory.

With this knowledge, invoke any of these scripts from your own by wrapping its invocation.

Example of `.s2i/bin/assemble` script:
```bash
#!/bin/bash
echo "Before assembling"

/usr/libexec/s2i/assemble
rc=$?

if [ $rc -eq 0 ]; then
    echo "After successful assembling"
else
    echo "After failed assembling"
fi

exit $rc
```

The example shows a custom `assemble` script that prints the message, executes standard `assemble` script from the image and prints another message depending on the exit code of the `assemble` script.

When wrapping the `run` script, you must [use `exec` for invoking it](https://docs.openshift.org/latest/creating_images/guidelines.html#general-docker-guidelines) to ensure signals are handled properly. Unfortunately, the use of `exec` also precludes the ability to run additional commands after invoking the default image run script.

Example of `.s2i/bin/run` script:
```bash
#!/bin/bash
echo "Before running application"
exec /usr/libexec/s2i/run
```
