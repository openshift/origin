package templates

const Dockerfile = `
# {{.ImageName}}
FROM centos:centos7

# TODO: Install required packages:
#
# RUN yum install -y --enablerepo=centosplus epel-release <packages>


# Add STI usage script
ADD ./.sti/bin/usage /usage

# You can add any other required files/configurations here
# ADD conf /app-root/conf

# TODO: Define the URL from where the default STI script will be fetched:
#
# ENV STI_SCRIPTS_URL https://raw.githubusercontent.com/<repository>/master/.sti/bin

# Default destination of scripts and sources, this is where assemble will look for them
ENV STI_LOCATION /tmp

# TODO: Create the application runtime user account and app-root directory:
RUN mkdir -p /app-root/src && \
    groupadd -r appuser -f -g 433 && \
    useradd -u 431 -r -g appuser -d /app-root -s /sbin/nologin -c "Application user" appuser && \
    chown -R appuser:appuser /app-root

# TODO: Specify the application root folder (if other than root folder), application user
#				and working directory.
#
ENV APP_ROOT .
ENV HOME     /app-root
ENV PATH     $HOME/bin:$PATH

WORKDIR     /app-root/src
USER        appuser

# TODO: Specify the default port your application is running on
EXPOSE 8080

CMD ["/usage"]
`
