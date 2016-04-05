"use strict";

describe('entrypointFilter', function(){
  var entrypoint, container, image;
  beforeEach(inject(function(entrypointFilter) {
    entrypoint = entrypointFilter;

    // Only `command` and `args` are looked at by the filter, which we set below for some tests.
    container = {};

    // Only `dockerImageMetadata.Config.Entrypoint` and `dockerImageMetadata.Config.Cmd` are looked at by the filter.
    image = {
      dockerImageMetadata: {
        Config: {
          Cmd: [
            '/usr/libexec/s2i/run'
          ],
          Entrypoint: [
            "container-entrypoint"
          ],
        }
      }
    };
  }));

  it('should return null if container or image are not specified', function() {
    expect(entrypoint(null, null)).toBeNull();
    expect(entrypoint(container, null)).toBeNull();
    expect(entrypoint(null, image)).toBeNull();
  });

  it('should return null if no entrypoint', function() {
    expect(entrypoint({}, {})).toBeNull();
  });

  it('should return the container command if defined in exec form', function() {
    container.command = ['sleep', '100'];
    expect(entrypoint(container, image)).toBe('sleep 100');
  });

  it('should return the container command if defined in shell form', function() {
    container.command = 'sleep 100';
    expect(entrypoint(container, image)).toBe('sleep 100');
  });

  it('should return the container command and args if defined in exec form', function() {
    container.command = ['/bin/sh', '-c'];
    container.args = ['echo', 'hello'];
    expect(entrypoint(container, image)).toBe('/bin/sh -c echo hello');
  });

  it('should return the container command and args if defined in shell form', function() {
    container.command =  '/bin/sh -c';
    container.args = 'echo hello';
    expect(entrypoint(container, image)).toBe('/bin/sh -c echo hello');
  });

  it('should return the image entrypoint and cmd if specified in exec form', function() {
    expect(entrypoint({}, image)).toBe('container-entrypoint /usr/libexec/s2i/run');
  });

  it('should return the image entrypoint and cmd if specified in shell form', function() {
    image.dockerImageMetadata.Config.Entrypoint = 'container-entrypoint';
    image.dockerImageMetadata.Config.Cmd = '/usr/libexec/s2i/run';
    expect(entrypoint({}, image)).toBe('container-entrypoint /usr/libexec/s2i/run');
  });

  it('should return the image entrypoint and cmd if specified in shell form', function() {
    expect(entrypoint({}, image)).toBe('container-entrypoint /usr/libexec/s2i/run');
  });

  it('should return the image entrypoint and container args if specified in exec form', function() {
    container.args = ['echo', 'hello'];
    expect(entrypoint(container, image)).toBe('container-entrypoint echo hello');
  });

  it('should return the image entrypoint and container args if specified in shell form', function() {
    container.args = 'echo hello';
    expect(entrypoint(container, image)).toBe('container-entrypoint echo hello');
  });

  it('should default entrypoint to /bin/sh -c', function() {
    image.dockerImageMetadata.Config.Entrypoint = null;
    expect(entrypoint(container, image)).toBe('/bin/sh -c /usr/libexec/s2i/run');
  });
});
