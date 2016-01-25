#!/bin/bash

# This library contains helpful aliases for manipulating the output text to the terminal as
# well as functions for one-command augmented printing.

# os::util::text::reset resets the terminal output to default if it is called in a TTY
function os::util::text::reset() {
	if [[ -t 1 ]]; then
		tput sgr0
	fi
}

# os::util::text::bold sets the terminal output to bold text if it is called in a TTY
function os::util::text::bold() {
	if [[ -t 1 ]]; then
		tput bold
	fi
}

# os::util::text::red sets the terminal output to red text if it is called in a TTY
function os::util::text::red() {
	if [[ -t 1 ]]; then
		tput setaf 1
	fi
}

# os::util::text::green sets the terminal output to green text if it is called in a TTY
function os::util::text::green() {
	if [[ -t 1 ]]; then
		tput setaf 2
	fi
}

# os::util::text::blue sets the terminal output to blue text if it is called in a TTY
function os::util::text::blue() {
	if [[ -t 1 ]]; then
		tput setaf 4
	fi
}

# os::util::text::yellow sets the terminal output to yellow text if it is called in a TTY
function os::util::text::yellow() {
	if [[ -t 1 ]]; then
		tput setaf 11
	fi
}

# os::util::text::clear_last_line clears the text from the last line of output to the
# terminal and leaves the cursor on that line to allow for overwriting that text
# if it is called in a TTY
function os::util::text::clear_last_line() {
	if [[ -t 1 ]]; then 
		tput cuu 1
		tput el
	fi
}

# os::util::text::print_bold prints all input in bold text
function os::util::text::print_bold() {
	os::util::text::bold
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_red prints all input in red text
function os::util::text::print_red() {
	os::util::text::red
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_red_bold prints all input in bold red text
function os::util::text::print_red_bold() {
	os::util::text::red
	os::util::text::bold
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_green prints all input in green text
function os::util::text::print_green() {
	os::util::text::green
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_green_bold prints all input in bold green text
function os::util::text::print_green_bold() {
	os::util::text::green
	os::util::text::bold
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_blue prints all input in blue text
function os::util::text::print_blue() {
	os::util::text::blue
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_blue_bold prints all input in bold blue text
function os::util::text::print_blue_bold() {
	os::util::text::blue
	os::util::text::bold
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_yellow prints all input in yellow text
function os::util::text::print_yellow() {
	os::util::text::yellow
	echo "$@"
	os::util::text::reset
}

# os::util::text::print_yellow_bold prints all input in bold yellow text
function os::util::text::print_yellow_bold() {
	os::util::text::yellow
	os::util::text::bold
	echo "$@"
	os::util::text::reset
}