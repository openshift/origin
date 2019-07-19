context
=======
[![GoDoc](https://godoc.org/github.com/gorilla/context?status.svg)](https://godoc.org/github.com/gorilla/context)
[![CircleCI](https://circleci.com/gh/gorilla/context.svg?style=svg)](https://circleci.com/gh/gorilla/context)

> 👷 **This library is in maintenance mode.**

> ⚠⚠⚠ **Note** ⚠⚠⚠ gorilla/context, having been born well before `context.Context` existed, does not play well
> with the shallow copying of the request that [`http.Request.WithContext`](https://golang.org/pkg/net/http/#Request.WithContext) (added to net/http Go 1.7 onwards) performs.
>
> Using gorilla/context may lead to memory leaks under those conditions, as the pointers to each `http.Request` become "islanded" and will not be cleaned up when the response is sent.
>
> You should use the `http.Request.Context()` feature in Go 1.7.

gorilla/context is a general purpose registry for global request variables.

* It stores a `map[*http.Request]map[interface{}]interface{}` as a global singleton, and thus tracks variables by their HTTP request.
* Read the full documentation here: https://www.gorillatoolkit.org/pkg/context

### License

See the LICENSE file for details.
