#!/usr/bin/plackup

use strict;
use warnings;

use File::Basename qw(dirname);
use Cwd  qw(abs_path);

use lib (dirname abs_path $0) .'/lib';
use My::Test qw(test);

my $count = 0;
sub {
	return [
		200,
		['Content-Type' => 'text/plain'],
		[test($count++)],
	];
}
