FROM google/nodejs
RUN npm i -g raml2html
ADD . /data
CMD ["-i", "/data/openshift3.raml", "-o", "/data/openshift3.html"]
ENTRYPOINT ["raml2html"]
