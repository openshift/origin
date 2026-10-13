# Design proposal to implement [Use build source secret based on annotation on Secret](https://trello.com/c/NoVpS2OS/1059-5-use-build-source-secret-based-on-annotation-on-secret-builds)

(The card goal is for source builds to be able to discover a relevant pre-
existing Secret and attempt to use it when a source repo requires
authentication.  The intention is to enable this without adding yet more CLI
arguments to oc new-app/new-build.)


##Design proposal

- A well-known annotation key prefix
  *build.openshift.io/source-secret-match-uri-* is proposed.

- Any Secret object may contain one or more annotations whose key is prefixed
  by the above value.  To be valid, the value of each such annotation must
  represent an "optionally-wildcarded URI pattern" as defined below.

- If a BuildConfig object is created referencing a source repository and not
  referencing any sourceSecret, and one or more "relevant" (defined below)
  Secrets exist, a new admission controller will add to the new BuildConfig
  object a sourceSecret reference to the Secret object calculated to be "most
  relevant" (also defined below).

- "Optionally-wildcarded URI patterns" are proposed to be very similar to
  Chrome's [Match
  Patterns](https://developer.chrome.com/extensions/match_patterns).  Their
  intended purpose is to enable the annotation of any given Secret with URIs
  against which it is usable, including basic wildcarding.

  I believe these patterns provide the appropriate level of flexibility, are as
  intuitive as these things can reasonably be, cover relevant security
  considerations in their design, and can be implemented straightforwardly.

  Proposed differences from the spec referenced above include:
  - the '\<all_urls\>' pattern would not be implemented.
  - permitted schemes would be 'git', 'http', 'https' and 'ssh'.
  - the scheme wildcard would match all above schemes.

  Note that the empty pattern is invalid and as such matches no URI.

- A given Secret object is considered "relevant" if:

  1. its type matches the method of access to the source repository, AND

  2. any of its annotated "optionally-wildcarded URI patterns" match the source
     repository URI.

  Specifically, requirement 1 above defines that a basicauth Secret is not
  relevant to a source repository accessed via SSH, and a sshauth Secret is not
  relevant to a source repository accessed by means other than SSH).

- In the valid case that multiple Secrets are "relevant" to a given source
  repository URI, the Secret chosen is one whose matching URI pattern is
  longest.  This allows for overrides, e.g. in the following scenario where
  the Secret object *override-for-my-dev-servers* is "most relevant" to the URI
  https://mydev1.mycorp.com/.

```
- kind: Secret
  metadata:
    name: matches-all-corporate-servers-https-only
    annotations:
      build.openshift.io/source-secret-match-uri-1: https://*.mycorp.com/*
  ...

- kind: Secret
  metadata:
    name: override-for-my-dev-servers
    annotations:
      build.openshift.io/source-secret-match-uri-1: https://mydev1.mycorp.com/*
      build.openshift.io/source-secret-match-uri-2: https://mydev2.mycorp.com/*
  ...
```

  Note that although it happens to be the case that the annotations in the above
  example are numerically ordered, this has no relevance to the working of the
  proposed algorithm.

Expected/intended uses of this design proposal include:

- Mapping of one or more sshauth and/or basicauth Secret objects to one or more
  servers hosting source code repositories.
- Mapping of one or more opaque Secret objects to one or more servers hosting
  source code repositories, in order to use a custom CA certificate
  automatically for TLS connections to the servers in question.
