FROM centos/ruby-26-centos7
USER default
EXPOSE 8080
ENV RACK_ENV production
ENV RAILS_ENV production
COPY . /opt/app-root/src/
RUN scl enable rh-ruby26 "bundle install"
CMD ["scl", "enable", "rh-ruby26", "./run.sh"]

USER default
