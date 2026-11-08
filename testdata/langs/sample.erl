-module(shapes).
-export([area/1, perimeter/2]).
-include("defs.hrl").
-include_lib("kernel/include/file.hrl").
-import(lists, [map/2]).

-record(point, {x, y}).
-type color() :: red | green.

area({circle, R}) -> 3.14 * R * R.

perimeter(W, H) -> 2 * (W + H).

helper(X) -> X.
