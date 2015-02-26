// GhostDriver used by PhantomJS is not compatible with selenium versions > 2.43.1
// See issue: https://github.com/angular/protractor/issues/1512

var pconfig = require('protractor/config.json');

pconfig.webdriverVersions.selenium = '2.43.1';

require('fs').writeFile(
  'node_modules/protractor/config.json', JSON.stringify(pconfig)
);