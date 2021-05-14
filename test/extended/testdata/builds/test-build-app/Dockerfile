FROM registry.access.redhat.com/ubi8/ruby-27
USER default
EXPOSE 8080
ENV RACK_ENV production
ENV RAILS_ENV production
COPY . /opt/app-root/src/
RUN bundle install
CMD ["./run.sh"]

USER default
