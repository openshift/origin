package main

const helpLongMsg = `
NAME:
   %_COMMAND_NAME_% - Lists the best common ancestors of the two passed commit revisions

SYNOPSIS:
  usage: %_COMMAND_NAME_% <path> <commitRev> <commitRev>
     or: %_COMMAND_NAME_% <path> --independent <commitRev>...
     or: %_COMMAND_NAME_% <path> --is-ancestor <commitRev> <commitRev>

 params:
    <path>       Path to the git repository
    <commitRev>  Git revision as supported by go-git

DESCRIPTION:
    %_COMMAND_NAME_% finds the best common ancestor(s) between two commits. One common ancestor is better than another common ancestor if the latter is an ancestor of the former.
    A common ancestor that does not have any better common ancestor is a best common ancestor, i.e. a merge base. Note that there can be more than one merge base for a pair of commits.
    Commits that does not share a common history has no common ancestors.

OPTIONS:
    As the most common special case, specifying only two commits on the command line means computing the merge base between the given two commits.
    If there is no shared history between the passed commits, there won't be a merge-base, and the command will exit with status 1.

--independent
    List the subgroup from the passed commits, that cannot be reached from any other of the passed ones. In other words, it prints a minimal subset of the supplied commits with the same ancestors.

--is-ancestor
    Check if the first commit is an ancestor of the second one, and exit with status 0 if true, or with status 1 if not. Errors are signaled by a non-zero status that is not 1.

DISCUSSION:
    Given two commits A and B, %_COMMAND_NAME_% A B will output a commit which is the best common ancestor of both, what means that is reachable from both A and B through the parent relationship.

    For example, with this topology:

             o---o---o---o---B
            /       /
    ---3---2---o---1---o---A

    the merge base between A and B is 1.

    With the given topology 2 and 3 are also common ancestors of A and B, but they are not the best ones because they can be also reached from 1.

    When the history involves cross-cross merges, there can be more than one best common ancestor for two commits. For example, with this topology:

    ---1---o---A
        \ /
         X
        / \
    ---2---o---o---B

    When the history involves feature branches depending on other feature branches there can be also more than one common ancestor. For example:


           o---o---o
          /         \
         1---o---A   \
        /       /     \
    ---o---o---2---o---o---B

    In both examples, both 1 and 2 are merge-bases of A and B for each situation.
    Neither one is better than the other (both are best merge bases) because 1 cannot be reached from 2, nor the opposite.
`
