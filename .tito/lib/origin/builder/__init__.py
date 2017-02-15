"""
Code for building Origin
"""
import json

from tito.common import get_latest_commit, run_command
from tito.builder import Builder

from ..common import inject_os_git_vars

class OriginBuilder(Builder):
    """
    builder which defines 'commit' as the git hash prior to building

    Used For:
        - Packages that want to know the commit in all situations
    """
    def _get_tag_for_version(self, version):
        return "v{}".format(version)

    def _get_rpmbuild_dir_options(self):
        git_hash = get_latest_commit()
        cmd = 'source ./hack/lib/init.sh; os::build::ldflags'
        ldflags = run_command("bash -c '{0}'".format(cmd))

        return ('--define "_topdir %s" --define "_sourcedir %s" --define "_builddir %s" '
                '--define "_srcrpmdir %s" --define "_rpmdir %s" --define "ldflags %s" '
                '--define "commit %s" ' % (
                    self.rpmbuild_dir,
                    self.rpmbuild_sourcedir, self.rpmbuild_builddir,
                    self.rpmbuild_basedir, self.rpmbuild_basedir,
                    ldflags, git_hash))

    def _setup_test_specfile(self):
        if self.test and not self.ran_setup_test_specfile:
            super(OriginBuilder, self)._setup_test_specfile()

            inject_os_git_vars(self.spec_file)
            self._inject_bundled_deps()

    def _inject_bundled_deps(self):
            # Add bundled deps for Fedora Guidelines as per:
            # https://fedoraproject.org/wiki/Packaging:Guidelines#Bundling_and_Duplication_of_system_libraries
            provides_list = []
            with open("./Godeps/Godeps.json") as godeps:
                depdict = json.load(godeps)
                for bdep in [
                    (dep[u'ImportPath'], dep[u'Rev'])
                    for dep in depdict[u'Deps']
                ]:
                    provides_list.append(
                        "Provides: bundled(golang({0})) = {1}".format(
                            bdep[0],
                            bdep[1]
                        )
                    )

            # Handle this in python because we have hit the upper bounds of line
            # count for what we can pass into sed via subprocess because there
            # are so many bundled libraries.
            with open(self.spec_file, 'r') as spec_file_f:
                spec_file_lines = spec_file_f.readlines()
            with open(self.spec_file, 'w') as spec_file_f:
                for line in spec_file_lines:
                    if '### AUTO-BUNDLED-GEN-ENTRY-POINT' in line:
                            spec_file_f.write(
                                '\n'.join(
                                    [provides.replace('"', '').replace("'", '')
                                     for provides in provides_list]
                                )
                            )
                    else:
                        spec_file_f.write(line)

# vim:expandtab:autoindent:tabstop=4:shiftwidth=4:filetype=python:textwidth=0:
