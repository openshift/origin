# OpenShift Shell Style Guide

## Table of Contents
1. [Introduction](#introduction)
  1. [Which Shell To Use](#which-shell-to-use)
  2. [When to use Shell](#when-to-use-shell)
2. [Output](#output)
  1. [Standard Out and Standard Error](#standard-output-and-standard-error)
3. [Comments](#comments)
  1. [Header](#header)
  2. [Function Comments]($function-comments)
  3. [Implementation Comments](#implementation-comments)
4. [Formatting](#formatting)
  1. [Indentation](#indentation)
  2. [Line Length](#line-length)
  3. [Pipelines](#pipelines)
  4. [Loops](#loops)
  5. [Variable Expansion](#variable-expansion)
  6. [Quoting](#quoting)
5. [Other Guidelines](#other-guidelines)
  1. [Shell Script Safety](#shell-script-safety)
  2. [Passing Arguments](#passing-arguments)
  3. [Command Substitution](#command-substitution)
  4. [Tests](#tests)
  5. [Testing Strings](#testing-strings)
  6. [Eval](#eval)
  7. [Pipes to While](#pipes-to-while)
6. [Naming Conventions](#naming-conventions)
  1. [Functions](#functions)
  2. [Variable Names](#variable-names)
  3. [Readonly Variables](#readonly-variables)
  4. [Local Variables](#local-variables)

## Introduction

### Which Shell To Use
Bash should be the only shell used for OpenShift scripts. 

### When to use Shell
Shell scripts should only be written when creating small utility functions or wrapping executable functionality.

If a script begins to use arrays or other "advanced" shell features, it should be re-written in Go.

If a script gets longer than several hundred lines, it should be re-written in Go.

## Output

### Standard Out and Standard Error
Scripts should be careful to write to `stdout` whenever they are writing an error message, and `stdout` when they are not.

## Comments

### Header
Every file should begin with a short description of its contents. If additions are being made and this requires editing the header comment, it's most likely that the additions should have been made to their own file.

Example:
```sh
#!/bin/bash

# This file contains helpful aliases for manipulating the output text to the terminal as
# well as functions for one-command augmented printing.
```

### Function Comments
Any function that is either non-obvious or long should be documented. Any function in a library should be documented.

All function comments must contain:
 * the full name of the function (including namespaces) as the first entry on the first line of the function
 * a description of the function
 * a list of global variables used in the function
 * a list of arguments to the function
 * what the function returns, if it returns something other than the exit status of the last command run

Example:
```sh
# os::cmd::internal::expect_exit_code_run_grep runs a command and makes an assertion about its exit code
# and the contents of the command's output to 'stdout' and/or 'stderr' Output from the command is suppressed
# unless either `VERBOSE=1` or the test fails. 
# Globals:
#  - VERBOSE
# Arguments:
#  - 1: the command to run
#  - 2: the command result code evaluation function
#  - 3: the arguments to pass to 'grep'
#  - 4: the command output evaluation function
# Return:
#  0 if all assertions are met for the command
#  1 if any assertion is not met for he command
```

### Implementation Comments
Comments should appear for any code that is difficult to understand, techincal, non-obvious or otherwise interesting.

## Formatting

### Indentation
All files will be indented four spaces, no tabs. Alignment is used often for multi-line command and will not be consistent across developer workstations if tabs are used.

### Line Length
Lines should not be longer than 120 characters. If lines are longer than this, determine if it is the result of long pipelines and re-format as specified [below](#pipelines).

### Pipelines
Pipelines should be formatted one per line if they don't all fit into one line. If the pipeline fits on one line, leave it as one line. These rules apply to chains of commands linked with the pipe `|` or logical statements `&&` or `||`.

Example:
```sh
# Pipeline fits on one line
command1 | command2 | command3

# Pipeline doesn't fit on one line
command1 | \
    | command2 \
    | command3 \
    | command4
```

### Loops
Put `; then` or `; do` on the same line as the preceding `for`, `while`, or `if`. `else` should be on a separate line. Concluding statements `done` and `fi` should be on their own lines and aligned with the opening statements.

Example:
```sh
for item in "${items}"; do
    if [[ -d "${item}" ]]; then
        some_action "${item}"
    else
        some_other_action "${item}"
    fi
done
```

### Variable Expansion
Quote variables, prefer `"${var}"` when possible

Don't brace-quote single-character shell special characters or positional parameters unless necessary or if the brace-quotes aids in readability.

Example:
```sh
echo "Positional parameters: $1 $2 $3 ..."
echo "Special characters: $! $- $_ $? $# $* $@ $$ ..."

echo "Braces necessary for multi-digit positional parameters: ${100}"

echo "Braces useful for being explicit: ${1}0 is better than $10"

echo "Other variables should be quoted and braced: ${PATH}"
```

### Quoting
Quote strings containing variables, command substitutions, spaces or shell meta characters unless expansion is explicitly unwanted. 

Example:
```sh
# Quote command substitutions
output="$(some_command_producing_result with flags)"

# Quote variables
echo "${var}"

# Don't quote literal integers
val=100

# Single-quoting regex keeps it as-written
grep -E '[^h]+(ello)'
``` 

## Other Guidelines

### Shell Script Safety
All executible shell scripts *must* set the following flags:
```
set -o errexit
set -o nounset
set -o pipefail
```

### Passing Arguments
Always use `"$@"` over `"$*"`. `$@` and `$*` split on spaces, which will clobber arguments containing spaces. `"$*"` expands to one argument separated by spaces, which is often not at all what is required. Unless there is a good reason, use `"$@"`.

### Command Substitution
Use `$(command)` over `\`command\``. Nested back-ticks require escaping, while `$(command)` nests without problems.

### Tests
Use `[[` over `test` or `[`. `[[` doesn't allow for pathname expansions or word splitting and allows for regular expression matching.

### Testing Strings
Use `[[ -z "${stringvar}" ]]` or `[[ -n "${stringvar}" ]]` instead of comparing to `""`.

### Eval
`eval` should be avoided unless absolutely necessary. `eval` will expand variables and it may be very difficult to debug.

### Pipes to While
Piping into `while` can cause difficulties in debugging because of the implicit subshell that is generated for each loop. Pass input into the `while` loop explicitly.

## Naming Conventions

### Functions
Functions should be named with lower-case letters, using underscores to separate words. Libraries/packages are separated with `::`. Parentheses after the function name are required, as is the `function` keyword before it. Braces begin on the same line the function is declared on.

Example:
```sh
function os::subpackage::my_function_name {
	...
}
```

### Variable Names
All variables that have global scope must be in `ALL_CAPS` using underscores to separate words. 

Variables used in a script must be `all_lowercase` using underscores to separate words.

### Readonly Variables
Use `readonly` or to ensure read-only variables remain that way. Variables with global scope that are meant to be read but not written to *must* be declared with `readonly`. 

### Local Variables
Function-specific variables must be declared within the function using `local`. Declaration and assignment should be on different lines, as `local var="$(func)"` shadows the return code of `func` with the return code of the assignment and `local`. 
