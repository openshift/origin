This package contains files copied from the main [kubernetes/kubernetes repo](https://github.com/kubernetes/kubernetes).
The files are taken from cmd/hyperkube.

Version: 1.8

The following additional changes have been made to the files.
- The package name from k8s.io/kubernetes/cmd/hyperkube has been changed from  
main to hyperkube.
- The use of stretchr/testify in k8s.io/kubernetes/cmd/hyperkube/hyperkube_test.go  
has been replaced with similar assert calls to functions in service-catalog/test/util/assertions.go.
- In k8s.io/kubernetes/cmd/hyperkube/hyperkube.go, the code to print the  
version has been replaced with version code from service-catalog.
- In k8s.io/kubernetes/cmd/hyperkube/server.go, made exportable the name field  
of the Server type, renaming the field to ServerName to avoid conflict with  
the Name function.