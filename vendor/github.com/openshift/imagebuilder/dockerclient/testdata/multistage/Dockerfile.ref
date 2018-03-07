FROM busybox:latest
WORKDIR /
COPY --from=nginx:latest /etc/nginx/nginx.conf /var/tmp/
COPY dir /var/dir
RUN cat /var/tmp/nginx.conf

