#!/bin/bash

# This file contains helpful aliases for manipulating the output text to the terminal as
# well as functions for one-command augmented printing.

# os::text::reset resets the terminal output to default if it is called in a TTY
function os::text::reset() {
	if [ -t 1 ]; then
		tput sgr0
	fi
}

# os::text::bold sets the terminal output to bold text if it is called in a TTY
function os::text::bold() {
	if [ -t 1 ]; then
		tput bold
	fi
}

# os::text::red sets the terminal output to red text if it is called in a TTY
function os::text::red() {
	if [ -t 1 ]; then
		tput setaf 1
	fi
}

# os::text::green sets the terminal output to green text if it is called in a TTY
function os::text::green() {
	if [ -t 1 ]; then
		tput setaf 2
	fi
}

# os::text::blue sets the terminal output to blue text if it is called in a TTY
function os::text::blue() {
	if [ -t 1 ]; then
		tput setaf 4
	fi
}

# os::text::yellow sets the terminal output to yellow text if it is called in a TTY
function os::text::yellow() {
	if [ -t 1 ]; then
		tput setaf 11
	fi
}

# os::text::clear_last_line clears the text from the last line of output to the
# terminal and leaves the cursor on that line to allow for overwriting that text
# if it is called in a TTY
function os::text::clear_last_line() {
	if [ -t 1 ]; then 
		tput cuu 1
		tput el
	fi
}

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