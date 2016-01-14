FROM centos/ruby-22-centos7

USER root
ADD ./secret-dir /secrets
COPY ./secret2 /

RUN test -f /secrets/secret1 && echo -n "secret1=" && cat /secrets/secret1
RUN test -f /secret2 && echo -n "relative-secret2=" && cat /secret2
RUN rm -rf /secrets && rm -rf /secret2

CMD ["true"]
