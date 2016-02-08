exports.resize = function(width, height) {
  // Want a longer browser size since the screenshot reporter only grabs the visible window
  browser.driver.manage().window().setSize(width || 1024, height || 2048);
};

exports.clearStorage = function() {
  browser.executeScript('window.sessionStorage.clear();');
  browser.executeScript('window.localStorage.clear();');
}
