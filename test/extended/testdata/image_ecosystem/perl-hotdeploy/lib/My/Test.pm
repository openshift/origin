package My::Test;
use strict; 
use warnings;

use Exporter qw(import);
 
our @EXPORT_OK = qw(test);

sub test {
  print "initial value\n";
}

1;
