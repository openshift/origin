#!/usr/bin/perl
use strict;
use warnings;

use File::Basename qw(dirname);
use Cwd  qw(abs_path);

use lib (dirname abs_path $0) .'/lib';
use My::Test qw(test);
   
print qq(Content-type: text/plain\n\n);
     
test();

