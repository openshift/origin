FROM busybox AS builder
WORKDIR /usr
RUN echo "test" > /usr/a.txt

FROM busybox
COPY --from=builder ./a.txt /b.txt
RUN ls /b.txt