"""
Code for tagging Spacewalk/Satellite packages.
"""

from tito.common import get_latest_commit, run_command
from tito.tagger import VersionTagger

class EmbeddedCommitTagger(VersionTagger):
  """
  Tagger which defines a specfile global with the git hash at which the tag was
  created.

  Requires that your commit is written on one single line as:
  %global commit 460abe2a3abe0fa22ac96c551fe71c0fc36f7475

  Used For:
    - Packages that are to be built via dist-git and need to have the git hash
      available to them.
  """

  def _tag_release(self):
    """
    Tag a new release of the package, add specfile global named commit. (ie: x.y.z-r+1)
    """
    self._make_changelog()
    new_version = self._bump_version(release=True)
    git_hash = get_latest_commit()
    update_commit = "sed -i 's/^%%global commit .*$/%%global commit %s/' %s" % \
      (git_hash, self.spec_file)
    output = run_command(update_commit)


    self._check_tag_does_not_exist(self._get_new_tag(new_version))
    self._update_changelog(new_version)
    self._update_package_metadata(new_version)
