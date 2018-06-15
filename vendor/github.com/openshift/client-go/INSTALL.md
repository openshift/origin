# Installing client-go
It is described here how to include the APIs provided by this repo into
your (GoLang-)project. If it is desired to compile and run examples in
the context of this repo, follow the README files in the example folders.

## Dependency management

### Glide

[Glide](https://github.com/Masterminds/glide) is a popular dependency
management tool for Go. Here we describe steps which are similar to
[k8s client-go](https://github.com/kubernetes/client-go/blob/master/INSTALL.md).


First create a `glide.yaml` file at the root of your project:

```yaml
package: ( your project's import path ) # e.g. github.com/foo/bar
import:
- package: github.com/openshift/client-go
  #version: xxx #you can specify the version if needed
```

Second, add a Go file that imports `client-go` somewhere in your project,
otherwise `client-go`'s dependencies will not be added to your project's
vendor/. Then run the following command in the same directory as `glide.yaml`:

```sh
$ glide update --strip-vendor
### Or, for short
$ glide up -v
```

At this point, `k8s.io/client-go` should be added to your project's vendor/.
`client-go`'s dependencies should be flattened and be added to your project's
vendor/ as well.