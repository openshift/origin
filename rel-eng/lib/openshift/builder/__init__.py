"""
Code for building Openshift v3
"""

from tito.common import get_latest_commit, run_command, get_script_path
from tito.builder import Builder

class OpenshiftBuilder(Builder):
  """
  builder which defines 'commit' as the git hash prior to building

  Used For:
    - Packages that want to know the commit in all situations
  """

  def _get_rpmbuild_dir_options(self):
      git_hash = get_latest_commit()
      cmd = '. ./hack/common.sh ; echo $(os::build::ldflags)'
      ldflags = run_command('bash -c \'%s\''  % (cmd) )

      return ('--define "_topdir %s" --define "_sourcedir %s" --define "_builddir %s" '
            '--define "_srcrpmdir %s" --define "_rpmdir %s" --define "ldflags %s" '
            '--define "commit %s" ' % (
                self.rpmbuild_dir,
                self.rpmbuild_sourcedir, self.rpmbuild_builddir,
                self.rpmbuild_basedir, self.rpmbuild_basedir,
                ldflags, git_hash))
  def _setup_test_specfile(self):
      if self.test and not self.ran_setup_test_specfile:
          # If making a test rpm we need to get a little crazy with the spec
          # file we're building off. (note that this is a temp copy of the
          # spec) Swap out the actual release for one that includes the git
          # SHA1 we're building for our test package:
          setup_specfile_script = get_script_path("test-setup-specfile.pl")
          cmd = "%s %s %s %s %s-%s %s" % \
                  (
                      setup_specfile_script,
                      self.spec_file,
                      self.git_commit_id[:7],
                      self.commit_count,
                      self.project_name,
                      self.display_version,
                      self.tgz_filename,
                  )
          run_command(cmd)
          # Custom Openshift v3 stuff follows, everything above is the standard
          # builder
          cmd = '. ./hack/common.sh ; echo $(os::build::ldflags)'
          ldflags = run_command('bash -c \'%s\''  % (cmd) )
          update_ldflags = "sed -i 's|^%%global ldflags .*$|%%global ldflags %s|' %s" % \
            (ldflags, self.spec_file)
          output = run_command(update_ldflags)

          self.build_version += ".git." + str(self.commit_count) + "." + str(self.git_commit_id[:7])
          self.ran_setup_test_specfile = True

