"""
Code for tagging Openshift v3 packages
"""

from tito.common import get_latest_commit, run_command
from tito.tagger import VersionTagger

class OpenshiftTagger(VersionTagger):
  """
  Tagger which defines a specfile global 'commit' with the git hash at
  which the tag was created. This also defines ldflags by importing
  hack/common.sh and executing os::build::ldflags Setting %commit isn't
  currently used, but seems to be the norm for RPM packaging of golang apps.

  Requires that your commit is written on one single line as:
  %global commit 460abe2a3abe0fa22ac96c551fe71c0fc36f7475

  And that your ldflags are similarly on a single line, ie:
  %global ldflags -X foo -X bar

  Used For:
    - Openshift v3, probably not much else
  """

  def _tag_release(self):
    """
    Tag a new release of the package, add specfile global named commit. (ie:
    x.y.z-r+1) and ldflags from hack/common.sh os::build::ldflags
    """
    self._make_changelog()
    new_version = self._bump_version(release=True)
    git_hash = get_latest_commit()
    update_commit = "sed -i 's/^%%global commit .*$/%%global commit %s/' %s" % \
      (git_hash, self.spec_file)
    output = run_command(update_commit)

    cmd = '. ./hack/common.sh ; echo $(os::build::ldflags)'
    ldflags = run_command('bash -c \'%s\''  % (cmd) )
    update_ldflags = "sed -i 's|^%%global ldflags .*$|%%global ldflags %s|' %s" % \
           (ldflags, self.spec_file)
    output = run_command(update_ldflags)

    self._check_tag_does_not_exist(self._get_new_tag(new_version))
    self._update_changelog(new_version)
    self._update_package_metadata(new_version)
