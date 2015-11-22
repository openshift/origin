#!/bin/bash

# This file contains helpful aliases for manipulating the output text to the terminal as
# well as functions for one-command augmented printing.

shopt -s expand_aliases

# The following aliases provide more readable accessors to `tput`
alias  os::text::reset='tput sgr0'
alias   os::text::bold='tput bold'
alias    os::text::red='tput setaf 1'
alias  os::text::green='tput setaf 2'
alias   os::text::blue='tput setaf 4'
alias os::text::yellow='tput setaf 11'

# os::text::clear_last_line clears the text from the last line of output to the
# terminal and leaves the cursor on that line to allow for overwriting that text
alias os::text::clear_last_line='tput cuu 1; tput el'

# The following functions wrap the above aliases to allow one-command printing of augmented text

# os::text::print_bold prints all input in bold text
function os::text::print_bold() {
	os::text::bold
	echo "${*}"
	os::text::reset
}

# os::text::print_red prints all input in red text
function os::text::print_red() {
	os::text::red
	echo "${*}"
	os::text::reset
}

# os::text::print_red_bold prints all input in bold red text
function os::text::print_red_bold() {
	os::text::red
	os::text::bold
	echo "${*}"
	os::text::reset
}

# os::text::print_green prints all input in green text
function os::text::print_green() {
	os::text::green
	echo "${*}"
	os::text::reset
}

# os::text::print_green_bold prints all input in bold green text
function os::text::print_green_bold() {
	os::text::green
	os::text::bold
	echo "${*}"
	os::text::reset
}

# os::text::print_blue prints all input in blue text
function os::text::print_blue() {
	os::text::blue
	echo "${*}"
	os::text::reset
}

# os::text::print_blue_bold prints all input in bold blue text
function os::text::print_blue_bold() {
	os::text::blue
	os::text::bold
	echo "${*}"
	os::text::reset
}

# os::text::print_yellow prints all input in yellow text
function os::text::print_yellow() {
	os::text::yellow
	echo "${*}"
	os::text::reset
}

# os::text::print_yellow_bold prints all input in bold yellow text
function os::text::print_yellow_bold() {
	os::text::yellow
	os::text::bold
	echo "${*}"
	os::text::reset
}