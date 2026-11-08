import strutils, sequtils
from os import getEnv
include helpers

type
  Color* = enum
    Red, Green
  Person* = object
    name*: string
    age: int
  Hidden = object
    x: int

proc add*(a, b: int): int =
  a + b

proc secret(a: int): int = a

func double*(x: int): int = x * 2

const Pi* = 3.14
