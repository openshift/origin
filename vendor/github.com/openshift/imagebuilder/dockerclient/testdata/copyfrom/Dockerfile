FROM centos:7 as base
RUN mkdir -p /a/blah && touch /a/blah/1 /a/blah/2
FROM centos:7
COPY --from=base /a/blah/* /blah/
