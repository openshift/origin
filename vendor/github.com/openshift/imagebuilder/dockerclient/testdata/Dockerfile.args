FROM busybox

ENV FOO="value" TEST=$BAR
LABEL test="$FOO"
ARG BAR
ENV BAZ=$BAR
RUN echo $BAR