Tracking memory usage in Heketi
===============================

Tracing and profiling with `pprof` is intended for developers only. This can be
used for debugging memory leaks/consumption, slow flows through the code and so
on. Users will normally not want profiling enabled on their production systems.

To investigate memory allocations in Heketi, it is needed to enable the
profiling feature. This can be done by setting the `HEKETI_PROFILING`
environment variable to a non-empty string or by adding `"profiling": true` to
the `--config` file (`/etc/heketi/heketi.json` by default).

Enabling profiling makes standard Golang pprof endpoints available. For memory
allocations `/debug/pprof/heap` is most useful.

Capturing a snapshot of the current allocations in the Heketi service is pretty
simple. Inside a Heketi container the `curl` command can be used:

```
sh-4.4# cd /var/lib/heketi
sh-4.4# curl 'http://localhost:8080/debug/pprof/heap' > heketi_heap.pprof
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100  138k    0  138k    0     0  5307k      0 --:--:-- --:--:-- --:--:-- 5111k
```

The new `heketi_heap.pprof` file has now been written and can be opened by `go
tool pprof`

```
sh-4.4# go tool pprof heketi_heap.pprof
File: heketi
Build ID: 059f700e17a8784ed6f60251c868dd4b3e002e85
Type: inuse_space
Time: Oct 16, 2018 at 11:14am (UTC)
Entering interactive mode (type "help" for commands, "o" for options)
(pprof)
```

For working with the size of the allocations, it is helpful to set all sizes to
`megabytes`, otherwise `auto` is used as a `unit` and that would add human
readable B, kB, MB postfixes. This however is not useful for sorting with
scripts. So, set the `unit` to `megabytes` instead:

```
(pprof) unit=megabytes
```



```
(pprof) top
Showing nodes accounting for 172.42MB, 86.24% of 199.94MB total
Dropped 67 nodes (cum <= 1MB)
Showing top 10 nodes out of 93
      flat  flat%   sum%        cum   cum%
   49.51MB 24.76% 24.76%    50.51MB 25.26%  net/textproto.(*Reader).ReadMIMEHeader
   30.51MB 15.26% 40.02%    30.51MB 15.26%  net/http.(*Request).WithContext
   23.83MB 11.92% 51.94%    23.83MB 11.92%  github.com/heketi/heketi/vendor/github.com/gorilla/context.Set
   19.51MB  9.76% 61.69%    19.51MB  9.76%  reflect.mapassign
   18.07MB  9.04% 70.73%    18.07MB  9.04%  net/http.newBufioReader
    6.50MB  3.25% 73.98%     6.50MB  3.25%  reflect.unsafe_New
    6.50MB  3.25% 77.23%     6.50MB  3.25%  context.WithValue
    6.50MB  3.25% 80.48%     6.50MB  3.25%  encoding/json.(*decodeState).literalStore
       6MB  3.00% 83.49%     6.50MB  3.25%  net/textproto.(*Reader).ReadLine
    5.50MB  2.75% 86.24%    39.51MB 19.76%  github.com/heketi/heketi/vendor/github.com/dgrijalva/jwt-go.(*Parser).ParseWithClaims
```

```
(pprof) traces > traces.txt
Generating report in traces.txt
```

This `traces.txt` contains the allocations and the functions that lead to it.
For the `ReadMIMEHeader` that is on the top of the list, the three largest
allocations are at

```
sh-4.4# grep ReadMIMEHeader traces.txt | sort -n -r | head -n3
   21.01MB   net/textproto.(*Reader).ReadMIMEHeader
      15MB   net/textproto.(*Reader).ReadMIMEHeader
    3.50MB   net/textproto.(*Reader).ReadMIMEHeader
```

Of course other statistics may be important too. It is possible that a small
allocation is done many more times. A summary of that can easily be produced
too.

```
sh-4.4# grep ReadMIMEHeader traces.txt | sort -n -r | uniq -c
      1    21.01MB   net/textproto.(*Reader).ReadMIMEHeader
      1       15MB   net/textproto.(*Reader).ReadMIMEHeader
      1     3.50MB   net/textproto.(*Reader).ReadMIMEHeader
      1        3MB   net/textproto.(*Reader).ReadMIMEHeader
      2     2.50MB   net/textproto.(*Reader).ReadMIMEHeader
      1     1.50MB   net/textproto.(*Reader).ReadMIMEHeader
      1     0.50MB   net/textproto.(*Reader).ReadMIMEHeader
     18          0   net/textproto.(*Reader).ReadMIMEHeader
      3              net/textproto.(*Reader).ReadMIMEHeader
```

By opening the `traces.txt` file with a text editor and searching for the
allocation (search for `21.01MB`) the callstack for the allocation is found:

```
     bytes:  352B
   21.01MB   net/textproto.(*Reader).ReadMIMEHeader
             net/http.readRequest
             net/http.(*conn).readRequest
             net/http.(*conn).serve
```


.... and now to find out whatever that means!
