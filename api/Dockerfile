FROM google/nodejs
RUN npm i -g raml2html
ADD . /data
CMD ["-i", "/data/oov3.raml", "-o", "/data/oov3.html"]
ENTRYPOINT ["raml2html"]
