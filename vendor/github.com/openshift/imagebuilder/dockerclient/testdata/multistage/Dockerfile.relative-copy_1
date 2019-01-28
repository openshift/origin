FROM busybox AS builder
WORKDIR /usr
RUN echo "test" > /usr/a.txt

FROM busybox
COPY --from=builder ./a.txt /other/
RUN ls /other/a.txt