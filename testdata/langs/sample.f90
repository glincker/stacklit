module geometry
  use iso_fortran_env
  use math_utils, only: sq
  implicit none
  real, parameter :: pi = 3.14159

  type :: point
    real :: x
    real :: y
  end type point

contains

  function area(r) result(a)
    real, intent(in) :: r
    real :: a
    a = pi * r * r
  end function area

  subroutine greet(name)
    character(len=*), intent(in) :: name
    print *, name
  end subroutine greet
end module geometry

program main
  use geometry
end program main
