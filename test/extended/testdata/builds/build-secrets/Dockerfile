FROM centos/ruby-22-centos7

USER root
ADD ./secret-dir /secrets
COPY ./secret2 /
ADD ./config-dir /configs
COPY ./this /

# Create a shell script that will output secrets and configMaps when the image is run
RUN echo '#!/bin/sh' > /secret_report.sh
RUN echo '(test -f /secrets/secret1 && echo -n "secret1=" && cat /secrets/secret1)' >> /secret_report.sh
RUN echo '(test -f /secret2 && echo -n "relative-secret2=" && cat /secret2)' >> /secret_report.sh
RUN echo '(test -f /configs/foo && echo -n "foo=" && cat /configs/foo)' >> /secret_report.sh
RUN echo '(test -f /this && echo -n "relative-this=" && cat /this)' >> /secret_report.sh
RUN chmod 755 /secret_report.sh

CMD ["/bin/sh", "-c", "/secret_report.sh"]
