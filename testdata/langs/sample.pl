package My::Module;
use strict;
use warnings;
use List::Util qw(sum max);
use parent 'Base::Class';
require Foo::Bar;

sub new {
    my ($class, %args) = @_;
    return bless {}, $class;
}

sub add {
    my ($self, $a, $b) = @_;
    return $a + $b;
}

sub _private { 1 }

1;
