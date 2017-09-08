"""
Code for tagging Origin packages
"""

import os
import re
import shutil
import subprocess
import tempfile
import textwrap

from tito.common import (
    debug,
    find_git_root,
    get_latest_commit,
    run_command,
    get_latest_tagged_version,
    tag_exists_locally,
    tag_exists_remotely,
    head_points_to_tag,
    undo_tag,
    tito_config_dir,
)

from tito.compat import write
from tito.tagger import VersionTagger
from tito.exception import TitoException


class OriginTagger(VersionTagger):
    """
    Origin custom tagger. This tagger has several deviations from normal
    the normal tito tagger.

    ** Rather than versions being tagged %{name}-%{version}-%{release} they're
    tagged as v%{version} in order to preserve compatibility with origin build
    processes. This means you really should not attempt to use the release field
    for anything useful, it should probably always remain zero.

    ** RPM specfile global commit is updated with the git hash, this may be
    relevant and popular with other golang projects, so TODO: submit to tito
    upstream.

    Requires that your commit global is written on one single line like this:

    %global commit 460abe2a3abe0fa22ac96c551fe71c0fc36f7475

    ** RPM specfile global ldflags is updated with os::build::ldflags as generated
    by importing hack/common.sh this absolutely depends on the non standard
    version tagging outlined above. This is 100% openshift specific

    Requires that your ldflags global is written on one single line like this:
    %global ldflags -X foo -X bar

    NOTE: Does not work with --use-version as tito does not provide a way to
    override the forced version tagger, see
    https://github.com/dgoodwin/tito/pull/163


    Used For:
    - Origin, probably not much else
    """

    def _tag_release(self):
        """
        Tag a new release of the package, add specfile global named commit.
        (ie: x.y.z-r+1) and ldflags from hack/common.sh os::build::ldflags
        """
        self._make_changelog()
        new_version = self._bump_version()
        new_version = re.sub(r"-.*", "", new_version)
        git_hash = get_latest_commit()
        update_commit = \
            "sed -i 's/^%global commit .*$/%global commit {0}/' {1}".format(
                git_hash,
                self.spec_file
            )
        output = run_command(update_commit)

        ## Fixup os_git_vars
        cmd = '. ./hack/common.sh ; OS_ROOT=$(pwd) ; os::build::os_version_vars ; echo ${OS_GIT_COMMIT}'
        os_git_commit = run_command("bash -c '{0}'".format(cmd))
        cmd = '. ./hack/common.sh ; OS_ROOT=$(pwd) ; os::build::os_version_vars ; echo ${OS_GIT_VERSION}'
        os_git_version = run_command("bash -c '{0}'".format(cmd))
        os_git_version = os_git_version.replace('-dirty', '')
        cmd = '. ./hack/common.sh ; OS_ROOT=$(pwd) ; os::build::os_version_vars ; echo ${OS_GIT_MAJOR}'
        os_git_major = run_command("bash -c '{0}'".format(cmd))
        cmd = '. ./hack/common.sh ; OS_ROOT=$(pwd) ; os::build::os_version_vars ; echo ${OS_GIT_MINOR}'
        os_git_minor = run_command("bash -c '{0}'".format(cmd))
        print("OS_GIT_COMMIT::{0}".format(os_git_commit))
        print("OS_GIT_VERSION::{0}".format(os_git_version))
        print("OS_GIT_MAJOR::{0}".format(os_git_major))
        print("OS_GIT_MINOR::{0}".format(os_git_minor))
        update_os_git_vars = \
            "sed -i 's|^%global os_git_vars .*$|%global os_git_vars OS_GIT_TREE_STATE='clean' OS_GIT_VERSION={0} OS_GIT_COMMIT={1} OS_GIT_MAJOR={2} OS_GIT_MINOR={3}|' {4}".format(
                os_git_version,
                os_git_commit,
                os_git_major,
                os_git_minor,
                self.spec_file
            )
        output = run_command(update_os_git_vars)

        self._check_tag_does_not_exist(self._get_new_tag(new_version))
        self._update_changelog(new_version)
        self._update_package_metadata(new_version)

    def _get_new_tag(self, new_version):
        """ Returns the actual tag we'll be creating. """
        return "v{0}".format(new_version)

    def get_latest_tagged_version(package_name):
        """
        Return the latest git tag for this package in the current branch.
        Uses the info in .tito/packages/package-name.

        Returns None if file does not exist.
        """
        git_root = find_git_root()
        rel_eng_dir = os.path.join(git_root, tito_config_dir())
        file_path = "{0}/packages/{1}".format(rel_eng_dir, package_name)
        debug("Getting latest package info from: {0}".format(file_path))
        if not os.path.exists(file_path):
            return None

        output = run_command("awk '{ print $1 ; exit }' {0}".format(file_path))
        if output is None or output.strip() == "":
            error_out("Error looking up latest tagged version in: {0}".format(file_path))

        return output

    def _make_changelog(self):
        """
        Create a new changelog entry in the spec, with line items from git
        """
        if self._no_auto_changelog:
            debug("Skipping changelog generation.")
            return

        in_f = open(self.spec_file, 'r')
        out_f = open(self.spec_file + ".new", 'w')

        found_changelog = False
        for line in in_f.readlines():
            out_f.write(line)

            if not found_changelog and line.startswith("%changelog"):
                found_changelog = True

                old_version = get_latest_tagged_version(self.project_name)

                # don't die if this is a new package with no history
                if old_version is not None:
                    last_tag = "v%s" % (old_version)
                    output = self._generate_default_changelog(last_tag)
                else:
                    output = self._new_changelog_msg

                fd, name = tempfile.mkstemp()
                write(fd, "# Create your changelog entry below:\n")
                if self.git_email is None or \
                        (('HIDE_EMAIL' in self.user_config) and
                        (self.user_config['HIDE_EMAIL'] not in ['0', ''])):
                    header = "* {0} {1}\n".format(self.today, self.git_user)
                else:
                    header = "* {0} {1} <{2}>\n".format(
                        self.today,
                        self.git_user,
                        self.git_email
                    )

                write(fd, header)

                for cmd_out in output.split("\n"):
                    write(fd, "- ")
                    write(fd, "\n  ".join(textwrap.wrap(cmd_out, 77)))
                    write(fd, "\n")

                write(fd, "\n")

                if not self._accept_auto_changelog:
                    # Give the user a chance to edit the generated changelog:
                    editor = 'vi'
                    if "EDITOR" in os.environ:
                        editor = os.environ["EDITOR"]
                    subprocess.call(editor.split() + [name])

                os.lseek(fd, 0, 0)
                file = os.fdopen(fd)

                for line in file.readlines():
                    if not line.startswith("#"):
                        out_f.write(line)

                output = file.read()

                file.close()
                os.unlink(name)

        if not found_changelog:
            print("WARNING: no %%changelog section find in spec file. Changelog entry was not appended.")

        in_f.close()
        out_f.close()

        shutil.move(self.spec_file + ".new", self.spec_file)

    def _undo(self):
        """
        Undo the most recent tag.

        Tag commit must be the most recent commit, and the tag must not
        exist in the remote git repo, otherwise we report and error out.
        """
        tag = "v{0}".format(get_latest_tagged_version(self.project_name))
        print("Undoing tag: {0}".format(tag))
        if not tag_exists_locally(tag):
            raise TitoException(
                "Cannot undo tag that does not exist locally.")
        if not self.offline and tag_exists_remotely(tag):
            raise TitoException("Cannot undo tag that has been pushed.")

        # Tag must be the most recent commit.
        if not head_points_to_tag(tag):
            raise TitoException("Cannot undo if tag is not the most recent commit.")

        # Everything looks good:
        print
        undo_tag(tag)

# This won't do anything until tito supports configuring the forcedversion tagger
# See https://github.com/dgoodwin/tito/pull/163
class OriginForceVersionTagger(OriginTagger):
    """
    Tagger which forcibly updates the spec file to a version provided on the
    command line by the --use-version option.
    TODO: could this be merged into main taggers?
    """

    def _tag_release(self):
        """
        Tag a new release of the package.
        """
        self._make_changelog()
        new_version = self._bump_version(force=True)
        self._check_tag_does_not_exist(self._get_new_tag(new_version))
        self._update_changelog(new_version)
        self._update_setup_py(new_version)
        self._update_package_metadata(new_version)

# vim:expandtab:autoindent:tabstop=4:shiftwidth=4:filetype=python:textwidth=0:
