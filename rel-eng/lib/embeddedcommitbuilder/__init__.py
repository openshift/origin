"""
Code for tagging Spacewalk/Satellite packages.
"""

from tito.common import get_latest_commit, run_command
from tito.builder import Builder

class EmbeddedCommitBuilder(Builder):
  """
  builder which defines 'commit' as the git hash prior to building

  Used For:
    - Packages that want to know the commit in all situations
  """

  def _get_rpmbuild_dir_options(self):
      git_hash = get_latest_commit()
      return ('--define "_topdir %s" --define "_sourcedir %s" --define "_builddir %s" --define '
            '"_srcrpmdir %s" --define "_rpmdir %s" --define "commit %s" ' % (
                self.rpmbuild_dir,
                self.rpmbuild_sourcedir, self.rpmbuild_builddir,
                self.rpmbuild_basedir, self.rpmbuild_basedir,
                git_hash))
